package decisions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// https://developers.openai.com/api/reference/resources/decisions/methods/create
type Request struct {
	Model     string          `json:"model"`
	Input     json.RawMessage `json:"input"`
	Questions []Question      `json:"questions"`
}

type Question struct {
	Type         string   `json:"type"`
	Name         *string  `json:"name,omitempty"`
	Instructions *string  `json:"instructions"`
	Choices      []Choice `json:"choices,omitempty"`
	Levels       []Level  `json:"levels,omitempty"`
}

type Choice struct {
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
}

type Level struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

func (l *Level) UnmarshalJSON(data []byte) error {
	var value struct {
		Label       *string `json:"label"`
		Description string  `json:"description"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Label == nil {
		return errors.New("score level label is required")
	}
	l.Label, l.Description = *value.Label, value.Description
	return nil
}

type Response struct {
	Model   string   `json:"model"`
	Answers []Answer `json:"answers"`
	Usage   Usage    `json:"usage"`
}

type Answer struct {
	Type          string        `json:"type"`
	Name          *string       `json:"name"`
	Probability   *float64      `json:"probability,omitempty"`
	Choice        any           `json:"choice,omitempty"`
	Confidence    *float64      `json:"confidence,omitempty"`
	Score         *float64      `json:"score,omitempty"`
	Probabilities []Probability `json:"probabilities,omitempty"`
}

type Probability struct {
	Value       any      `json:"value"`
	Label       *string  `json:"label,omitempty"`
	Probability *float64 `json:"probability"`
}

type Usage struct {
	InputTokens        int `json:"input_tokens"`
	InputTokensDetails struct {
		CachedTokens     int `json:"cached_tokens"`
		CacheWriteTokens int `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
}

// ProviderInput validates the supported wire variants before adapting them.
func (r Request) ProviderInput() (*provider.DecisionInput, error) {
	if strings.TrimSpace(r.Model) == "" {
		return nil, errors.New("model is required")
	}
	if err := validateInput(r.Input); err != nil {
		return nil, err
	}
	input := &provider.DecisionInput{State: r.Input}
	for i, q := range r.Questions {
		if q.Instructions == nil {
			return nil, fmt.Errorf("questions[%d]: instructions is required", i)
		}
		question := provider.DecisionQuestion{ID: strconv.Itoa(i), Instructions: *q.Instructions}
		switch q.Type {
		case "predicate":
			if len(q.Choices) > 0 || len(q.Levels) > 0 {
				return nil, errors.New("predicate does not accept choices or levels")
			}
			question.Noul = &provider.NoulQuestion{}
		case "choice":
			if len(q.Levels) > 0 {
				return nil, errors.New("choice does not accept levels")
			}
			question.Choice = &provider.ChoiceQuestion{}
			seen := make(map[any]bool)
			for j, c := range q.Choices {
				switch c.Value.(type) {
				case string, bool:
				default:
					return nil, errors.New("choice value must be a string or boolean")
				}
				if seen[c.Value] {
					return nil, errors.New("duplicate choice value")
				}
				seen[c.Value] = true
				question.Choice.Options = append(question.Choice.Options, provider.DecisionOption{Label: strconv.Itoa(j), Value: c.Value, Description: c.Description})
			}
		case "score":
			if len(q.Choices) > 0 {
				return nil, errors.New("score does not accept choices")
			}
			question.Score = &provider.ScoreQuestion{}
			for _, l := range q.Levels {
				question.Score.Levels = append(question.Score.Levels, l.Description)
				question.Score.Labels = append(question.Score.Labels, l.Label)
			}
		default:
			return nil, fmt.Errorf("questions[%d]: unsupported type %q", i, q.Type)
		}
		input.Questions = append(input.Questions, question)
	}
	return input, input.Validate()
}

func validateInput(raw json.RawMessage) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("input is required")
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return nil
	}
	var messages []struct {
		Role    string          `json:"role"`
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil || len(messages) == 0 {
		return errors.New("input must be text or user messages")
	}
	images := 0
	for _, m := range messages {
		if m.Role != "user" || (m.Type != "" && m.Type != "message") {
			return errors.New("only user messages are supported")
		}
		if len(m.Content) == 0 || bytes.Equal(bytes.TrimSpace(m.Content), []byte("null")) {
			return errors.New("message content is required")
		}
		if json.Unmarshal(m.Content, &text) == nil {
			continue
		}
		var parts []struct {
			Type     string  `json:"type"`
			Text     *string `json:"text"`
			ImageURL string  `json:"image_url"`
			Detail   string  `json:"detail"`
		}
		if json.Unmarshal(m.Content, &parts) != nil || len(parts) == 0 {
			return errors.New("content must be text or input parts")
		}
		for _, p := range parts {
			switch p.Type {
			case "input_text":
				if p.Text == nil {
					return errors.New("input_text requires text")
				}
			case "input_image":
				images++
				prefix, data, ok := strings.Cut(p.ImageURL, ",")
				if !ok || !strings.HasPrefix(prefix, "data:image/") || !strings.HasSuffix(prefix, ";base64") || data == "" {
					return errors.New("images require a base64 image data URL")
				}
				if _, err := base64.StdEncoding.DecodeString(data); err != nil {
					return errors.New("invalid image data URL")
				}
				switch p.Detail {
				case "", "low", "high", "auto", "original":
				default:
					return errors.New("invalid image detail")
				}
			default:
				return errors.New("only input_text and input_image parts are supported")
			}
		}
	}
	if images > 128 {
		return errors.New("at most 128 images are supported")
	}
	return nil
}
