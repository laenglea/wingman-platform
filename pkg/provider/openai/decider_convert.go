package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/openai/openai-go/v3"
)

func (d *Decider) convertDecisionRequest(input *provider.DecisionInput) (*openai.DecisionNewParams, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	req := &openai.DecisionNewParams{Model: d.model}
	raw, _ := json.Marshal(input.State)
	// System One also accepts structured state; serialize it as text. Preserve
	// user message arrays from the Decisions endpoint as multimodal input.
	var messages []struct {
		Role string `json:"role"`
	}
	hasMessages := json.Unmarshal(raw, &messages) == nil && len(messages) > 0
	if hasMessages {
		for _, m := range messages {
			if m.Role != "user" {
				hasMessages = false
				break
			}
		}
	}
	if hasMessages {
		if err := json.Unmarshal(raw, &req.Input.OfDecisionInputMessageArray); err != nil {
			return nil, err
		}
	} else {
		req.Input.OfString = openai.String(decisionText(input.State))
	}
	for _, q := range input.Questions {
		instructions := decisionText(q.Instructions)
		name := openai.String(q.ID)
		switch {
		case q.Noul != nil:
			if q.Noul.True != nil || q.Noul.False != nil {
				instructions += "\nCriteria: " + decisionText(map[string]any{"true": q.Noul.True, "false": q.Noul.False})
			}
			req.Questions = append(req.Questions, openai.DecisionNewParamsQuestionUnion{OfPredicate: &openai.DecisionNewParamsQuestionPredicate{
				Name: name, Instructions: instructions,
			}})
		case q.Choice != nil:
			question := &openai.DecisionNewParamsQuestionChoice{Name: name, Instructions: instructions}
			for _, o := range q.Choice.Options {
				value := openai.DecisionNewParamsQuestionChoiceChoiceValueUnion{OfString: openai.String(o.Label)}
				switch v := o.Value.(type) {
				case string:
					value.OfString = openai.String(v)
				case bool:
					value = openai.DecisionNewParamsQuestionChoiceChoiceValueUnion{OfBool: openai.Bool(v)}
				}
				question.Choices = append(question.Choices, openai.DecisionNewParamsQuestionChoiceChoice{
					Value: value, Description: openai.String(decisionText(o.Description)),
				})
			}
			req.Questions = append(req.Questions, openai.DecisionNewParamsQuestionUnion{OfChoice: question})
		case q.Score != nil:
			question := &openai.DecisionNewParamsQuestionScore{Name: name, Instructions: instructions}
			for j, level := range q.Score.Levels {
				label := strconv.Itoa(j)
				if q.Score.Labels != nil {
					label = q.Score.Labels[j]
				}
				question.Levels = append(question.Levels, openai.DecisionNewParamsQuestionScoreLevel{Label: label, Description: openai.String(decisionText(level))})
			}
			req.Questions = append(req.Questions, openai.DecisionNewParamsQuestionUnion{OfScore: question})
		}
	}
	return req, nil
}

// Values have already passed DecisionInput.Validate, including JSON encoding.
func decisionText(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	data, _ := json.Marshal(value)
	var text string
	if json.Unmarshal(data, &text) == nil {
		return text
	}
	return string(data)
}

func convertDecision(decision *openai.Decision, input *provider.DecisionInput) (*provider.Decision, error) {
	if decision == nil || decision.Model == "" || len(decision.Answers) != len(input.Questions) {
		return nil, errors.New("incomplete decision response")
	}
	result := &provider.Decision{Model: decision.Model, Usage: &provider.Usage{
		InputTokens: int(decision.Usage.InputTokens), OutputTokens: int(decision.Usage.OutputTokens),
		CacheReadInputTokens:     int(decision.Usage.InputTokensDetails.CachedTokens),
		CacheCreationInputTokens: int(decision.Usage.InputTokensDetails.CacheWriteTokens),
	}}
	if decision.Usage.OutputTokensDetails.JSON.ReasoningTokens.Valid() {
		tokens := int(decision.Usage.OutputTokensDetails.ReasoningTokens)
		result.Usage.ReasoningTokens = &tokens
	}
	for i, answer := range decision.Answers {
		q := input.Questions[i]
		if answer.Name != q.ID {
			return nil, errors.New("decision answer order does not match questions")
		}
		value := provider.DecisionAnswer{ID: q.ID}
		switch a := answer.AsAny().(type) {
		case openai.DecisionAnswerPredicate:
			if q.Noul == nil || !a.JSON.Probability.Valid() {
				return nil, errors.New("invalid predicate answer")
			}
			value.Noul = &provider.NoulAnswer{Probability: a.Probability}
		case openai.DecisionAnswerChoice:
			if q.Choice == nil || len(a.Probabilities) != len(q.Choice.Options) {
				return nil, errors.New("invalid choice answer")
			}
			choice := any(a.Choice.OfString)
			if a.Choice.JSON.OfBool.Valid() {
				choice = a.Choice.OfBool
			}
			distribution := make(map[string]float64, len(a.Probabilities))
			selected := false
			value.Choice = &provider.ChoiceAnswer{Confidence: a.Confidence, Probabilities: distribution}
			for _, p := range a.Probabilities {
				optionValue := any(p.Value.OfString)
				if p.Value.JSON.OfBool.Valid() {
					optionValue = p.Value.OfBool
				}
				for _, option := range q.Choice.Options {
					expected := option.Value
					if expected == nil {
						expected = option.Label
					}
					if optionValue != expected {
						continue
					}
					distribution[option.Label] = p.Probability
					if choice == expected {
						value.Choice.Choice, selected = option.Label, true
					}
				}
			}
			if !selected || len(distribution) != len(q.Choice.Options) {
				return nil, errors.New("unknown decision choice")
			}
		case openai.DecisionAnswerScore:
			if q.Score == nil || len(a.Probabilities) != len(q.Score.Levels) {
				return nil, errors.New("invalid score answer")
			}
			value.Score = &provider.ScoreAnswer{Score: a.Score, Confidence: a.Confidence, Levels: make([]provider.ScoreLevel, len(q.Score.Levels))}
			seen := make(map[int64]bool)
			for _, p := range a.Probabilities {
				if p.Value < 0 || p.Value >= int64(len(q.Score.Levels)) || seen[p.Value] {
					return nil, errors.New("invalid score level")
				}
				seen[p.Value] = true
				value.Score.Levels[p.Value] = provider.ScoreLevel{Description: q.Score.Levels[p.Value], Probability: p.Probability}
			}
		case openai.DecisionAnswerRefusal:
			value.Refusal = &provider.RefusalAnswer{}
		default:
			return nil, fmt.Errorf("unsupported decision answer: %s", answer.Type)
		}
		result.Answers = append(result.Answers, value)
	}
	return result, nil
}
