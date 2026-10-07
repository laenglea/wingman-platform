package decisions

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// FromDecision restores caller names and typed values for native or adapted deciders.
func FromDecision(request Request, input *provider.DecisionInput, result *provider.Decision) (Response, error) {
	if result == nil || len(result.Answers) != len(input.Questions) {
		return Response{}, errors.New("incomplete decision response")
	}
	response := Response{Model: result.Model, Answers: make([]Answer, 0, len(result.Answers))}
	if response.Model == "" {
		response.Model = request.Model
	}
	if u := result.Usage; u != nil {
		response.Usage.InputTokens, response.Usage.OutputTokens = u.InputTokens, u.OutputTokens
		response.Usage.TotalTokens = u.InputTokens + u.OutputTokens
		response.Usage.InputTokensDetails.CachedTokens = u.CacheReadInputTokens
		response.Usage.InputTokensDetails.CacheWriteTokens = u.CacheCreationInputTokens
		if u.ReasoningTokens != nil {
			response.Usage.OutputTokensDetails.ReasoningTokens = *u.ReasoningTokens
		}
	}
	for i, a := range result.Answers {
		wire, q := request.Questions[i], input.Questions[i]
		if a.ID != q.ID {
			return Response{}, errors.New("decision answers must match question order")
		}
		value := Answer{Name: wire.Name}
		switch {
		case a.Refusal != nil:
			value.Type = "refusal"
		case a.Noul != nil && wire.Type == "predicate":
			value.Type, value.Probability = "predicate", &a.Noul.Probability
		case a.Choice != nil && wire.Type == "choice":
			value.Type, value.Confidence = "choice", &a.Choice.Confidence
			selected := false
			for j, choice := range wire.Choices {
				label := q.Choice.Options[j].Label
				p, ok := a.Choice.Probabilities[label]
				if !ok {
					return Response{}, errors.New("missing choice probability")
				}
				value.Probabilities = append(value.Probabilities, Probability{Value: choice.Value, Probability: &p})
				if a.Choice.Choice == label {
					value.Choice, selected = choice.Value, true
				}
			}
			if !selected {
				return Response{}, errors.New("unknown selected choice")
			}
		case a.Score != nil && wire.Type == "score":
			if len(a.Score.Levels) != len(wire.Levels) {
				return Response{}, errors.New("missing score levels")
			}
			value.Type, value.Score, value.Confidence = "score", &a.Score.Score, &a.Score.Confidence
			for j, level := range wire.Levels {
				label := level.Label
				value.Probabilities = append(value.Probabilities, Probability{Value: float64(j), Label: &label, Probability: &a.Score.Levels[j].Probability})
			}
		default:
			return Response{}, fmt.Errorf("question %s: invalid answer variant", strconv.Itoa(i))
		}
		response.Answers = append(response.Answers, value)
	}
	return response, nil
}
