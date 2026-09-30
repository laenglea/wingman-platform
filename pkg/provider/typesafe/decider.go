package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
)

var _ provider.Decider = (*Decider)(nil)

type Decider struct {
	*Config
}

// NewDecider accepts a complete evaluation endpoint, including its path.
// An empty endpoint uses TypeSafe's hosted System One API.
func NewDecider(endpoint, model string, options ...Option) (*Decider, error) {
	if endpoint == "" {
		endpoint = "https://api.typesafe.ai/v1/systemone"
	}
	cfg := &Config{endpoint: endpoint, model: model, maxRetries: 2}
	for _, option := range options {
		option(cfg)
	}
	if cfg.client == nil {
		cfg.client = provider.DefaultClient
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("decision model is required")
	}
	return &Decider{Config: cfg}, nil
}

type decisionRequest struct {
	Model     string                      `json:"model"`
	State     any                         `json:"state"`
	Questions map[string]decisionQuestion `json:"questions"`
}

type decisionQuestion struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type decisionResponse struct {
	ID      string                    `json:"id"`
	Model   string                    `json:"model"`
	Answers map[string]decisionAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type decisionAnswer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        *string            `json:"choice"`
	Score         *float64           `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
	Legend        map[string]any     `json:"legend"`
}

func (d *Decider) Decide(ctx context.Context, input *provider.DecisionInput) (*provider.Decision, error) {
	if err := input.Validate(); err != nil {
		return nil, provider.InvalidRequest(err)
	}
	body := decisionRequest{
		Model: d.model, State: input.State,
		Questions: make(map[string]decisionQuestion, len(input.Questions)),
	}
	for _, q := range input.Questions {
		question := decisionQuestion{Instructions: q.Instructions}
		switch {
		case q.Noul != nil:
			question.Type = "noul"
			criteria := make(map[string]any)
			if q.Noul.True != nil {
				criteria["true"] = q.Noul.True
			}
			if q.Noul.False != nil {
				criteria["false"] = q.Noul.False
			}
			if len(criteria) > 0 {
				question.Criteria = criteria
			}
		case q.Choice != nil:
			question.Type = "choice"
			criteria := make(map[string]any, len(q.Choice.Options))
			for _, option := range q.Choice.Options {
				criteria[option.Label] = option.Description
			}
			question.Criteria = criteria
		case q.Score != nil:
			question.Type, question.Criteria = "score", q.Score.Levels
		}
		body.Questions[q.ID] = question
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, provider.InvalidRequest(err)
	}

	var response *decisionResponse
	for attempt := 0; ; attempt++ {
		response, err = d.evaluate(ctx, data)
		if err == nil || attempt >= d.maxRetries {
			break
		}
		var upstream *provider.ProviderError
		if !errors.As(err, &upstream) || !(upstream.Code == 408 || upstream.Code == 429 || upstream.Code >= 500) {
			break
		}
		delay := upstream.RetryAfter
		if delay <= 0 {
			delay = time.Duration(1<<min(attempt, 4)) * 500 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if err != nil {
		return nil, err
	}
	return response.decision(input)
}

func (d *Decider) evaluate(ctx context.Context, data []byte) (*decisionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		upstream := &provider.ProviderError{Code: resp.StatusCode, Message: string(data)}
		var envelope struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Error.Message != "" {
			upstream.Message, upstream.Type = envelope.Error.Message, envelope.Error.Type
		}
		if upstream.Message == "" {
			upstream.Message = fmt.Sprintf("decision API returned HTTP %d", resp.StatusCode)
		}
		if seconds, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64); err == nil && seconds > 0 && !math.IsInf(seconds, 0) {
			upstream.RetryAfter = time.Duration(seconds * float64(time.Second))
		} else if deadline, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil {
			upstream.RetryAfter = max(0, time.Until(deadline))
		}
		return nil, upstream
	}

	var result decisionResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid decision response: %w", err)
	}
	return &result, nil
}

func (r *decisionResponse) decision(input *provider.DecisionInput) (*provider.Decision, error) {
	if r.Model == "" || len(r.Answers) != len(input.Questions) {
		return nil, errors.New("decision response must contain a model and every question exactly once")
	}
	result := &provider.Decision{
		ID: r.ID, Model: r.Model,
		Usage: &provider.Usage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens},
	}
	for _, q := range input.Questions {
		answer, ok := r.Answers[q.ID]
		if !ok {
			return nil, fmt.Errorf("decision response is missing question %q", q.ID)
		}
		value := provider.DecisionAnswer{ID: q.ID}
		switch {
		case q.Noul != nil:
			if answer.Type != "noul" || !probability(answer.Noul) {
				return nil, fmt.Errorf("question %q: invalid noul answer", q.ID)
			}
			value.Noul = &provider.NoulAnswer{Probability: *answer.Noul}
		case q.Choice != nil:
			if answer.Type != "choice" || answer.Choice == nil || !probability(answer.Confidence) || len(answer.Probabilities) != len(q.Choice.Options) {
				return nil, fmt.Errorf("question %q: invalid choice answer", q.ID)
			}
			if _, ok := answer.Probabilities[*answer.Choice]; !ok {
				return nil, fmt.Errorf("question %q: unknown selected choice", q.ID)
			}
			for _, option := range q.Choice.Options {
				p, ok := answer.Probabilities[option.Label]
				if !ok || !probability(&p) {
					return nil, fmt.Errorf("question %q: invalid probability for choice %q", q.ID, option.Label)
				}
			}
			value.Choice = &provider.ChoiceAnswer{
				Choice: *answer.Choice, Confidence: *answer.Confidence, Probabilities: answer.Probabilities,
			}
		case q.Score != nil:
			if answer.Type != "score" || answer.Score == nil || *answer.Score < 0 || *answer.Score > float64(len(q.Score.Levels)-1) || !probability(answer.Confidence) ||
				len(answer.Probabilities) != len(q.Score.Levels) || len(answer.Legend) != len(q.Score.Levels) {
				return nil, fmt.Errorf("question %q: invalid score answer", q.ID)
			}
			score := &provider.ScoreAnswer{Score: *answer.Score, Confidence: *answer.Confidence}
			for i := range q.Score.Levels {
				key := strconv.Itoa(i)
				p, ok := answer.Probabilities[key]
				description, exists := answer.Legend[key]
				if !ok || !exists || !probability(&p) {
					return nil, fmt.Errorf("question %q: invalid score level %q", q.ID, key)
				}
				score.Levels = append(score.Levels, provider.ScoreLevel{Description: description, Probability: p})
			}
			value.Score = score
		}
		result.Answers = append(result.Answers, value)
	}
	return result, nil
}

func probability(value *float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0 && *value <= 1
}
