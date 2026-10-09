package anthropic

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// Exercise the encoded requests, including the policies that differ from 5.
func TestModels55ThinkingPolicy(t *testing.T) {
	for _, model := range []string{"claude-sonnet-5-5", "claude-opus-5-5"} {
		t.Run(model, func(t *testing.T) {
			c, _ := NewCompleter("http://localhost", model)
			input := []provider.Message{provider.UserMessage("Hi")}
			body := requestBody(t, c, input, nil)
			if body["thinking"] != nil || body["output_config"] != nil || body["max_tokens"] != float64(128000) {
				t.Fatalf("changed model defaults: %v", body)
			}
			for _, field := range []string{"temperature", "top_p", "top_k"} {
				if _, exists := body[field]; exists {
					t.Fatalf("unsupported sampling parameter %s", field)
				}
			}
			body = requestBody(t, c, input, &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{IncludeSummary: true}})
			thinking := body["thinking"].(map[string]any)
			if thinking["type"] != "adaptive" || thinking["display"] != "summarized" {
				t.Fatalf("summary = %v", thinking)
			}

			body = requestBody(t, c, input, &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeDisabled, Effort: provider.EffortMax}})
			effort := body["output_config"].(map[string]any)["effort"]
			if model == "claude-opus-5-5" {
				if body["thinking"] != nil || effort != "max" {
					t.Fatalf("Opus must keep thinking on at requested effort: %v", body)
				}
			} else {
				thinking := body["thinking"].(map[string]any)
				if thinking["type"] != "between_tools" || len(thinking) != 1 || effort != "high" {
					t.Fatalf("Sonnet must use between_tools at high or below: %v", body)
				}
			}
		})
	}
}
