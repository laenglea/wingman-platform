package decisions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/server/openai/shared"
)

// https://developers.openai.com/api/reference/resources/decisions/methods/create
type Request struct {
	Model              string                     `json:"model"`
	Input              json.RawMessage            `json:"input"`
	Questions          []Question                 `json:"questions"`
	PromptCacheKey     *string                    `json:"prompt_cache_key,omitempty"`
	PromptCacheOptions *shared.PromptCacheOptions `json:"prompt_cache_options,omitempty"`
}

type Question struct {
	Type         string          `json:"type"`
	Name         *string         `json:"name,omitempty"`
	Instructions json.RawMessage `json:"instructions"`
	Choices      []Choice        `json:"choices,omitempty"`
	Levels       []Level         `json:"levels,omitempty"`
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
		CachedTokens            int                             `json:"cached_tokens"`
		CacheWriteTokens        int                             `json:"cache_write_tokens"`
		CacheWriteTokensDetails *shared.CacheWriteTokensDetails `json:"cache_write_tokens_details,omitempty"`
	} `json:"input_tokens_details"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
}

// ProviderInput validates the supported wire variants before adapting them.
func (r Request) ProviderInput() (*provider.DecisionInput, error) {
	if err := validateInput(r.Input); err != nil {
		return nil, err
	}
	input := &provider.DecisionInput{State: r.Input}
	if err := r.PromptCacheOptions.Validate(); err != nil {
		return nil, err
	}
	input.CacheOptions = shared.CacheOptions(r.PromptCacheKey, nil, r.PromptCacheOptions)
	if len(r.Questions) > 400 {
		return nil, errors.New("at most 400 questions are supported")
	}
	for i, q := range r.Questions {
		if len(q.Instructions) == 0 || bytes.Equal(bytes.TrimSpace(q.Instructions), []byte("null")) {
			return nil, fmt.Errorf("questions[%d]: instructions is required", i)
		}
		if err := validateParts(q.Instructions, "question"); err != nil {
			return nil, fmt.Errorf("questions[%d].instructions: %w", i, err)
		}
		question := provider.DecisionQuestion{ID: strconv.Itoa(i), Instructions: q.Instructions}
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
