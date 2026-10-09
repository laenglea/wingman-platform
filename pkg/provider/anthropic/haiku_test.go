package anthropic

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestHaiku55Request(t *testing.T) {
	c, _ := NewCompleter("http://localhost", "claude-haiku-5-5")
	for _, tc := range []struct {
		name, thinking, display, effort string
		opts                            provider.CompleteOptions
	}{
		{name: "default"},
		{name: "summary", thinking: "adaptive", display: "summarized", opts: provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{IncludeSummary: true}}},
		{name: "disabled", thinking: "disabled", effort: "high", opts: provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeDisabled, Effort: provider.EffortXHigh}}},
		{name: "forced", thinking: "disabled", effort: "high", opts: provider.CompleteOptions{
			ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeAdaptive, Effort: provider.EffortMax},
			Tools:            []provider.Tool{{Name: "lookup", Parameters: map[string]any{"type": "object"}}},
			ToolOptions:      &provider.ToolOptions{Choice: provider.ToolChoiceAny, Allowed: []string{"lookup"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := requestBody(t, c, []provider.Message{provider.UserMessage("Hi")}, &tc.opts)
			if body["max_tokens"] != float64(128000) {
				t.Fatalf("output limit = %v", body["max_tokens"])
			}
			thinking, _ := body["thinking"].(map[string]any)
			if tc.thinking == "" {
				if thinking != nil {
					t.Fatalf("default thinking must stay implicit: %v", thinking)
				}
			} else if thinking["type"] != tc.thinking || (tc.display != "" && thinking["display"] != tc.display) {
				t.Fatalf("thinking = %v", thinking)
			}
			if tc.effort != "" && body["output_config"].(map[string]any)["effort"] != tc.effort {
				t.Fatalf("effort = %v", body["output_config"])
			}
			for _, field := range []string{"temperature", "top_p", "top_k"} {
				if _, exists := body[field]; exists {
					t.Fatalf("unsupported sampling parameter %s", field)
				}
			}
		})
	}
}

func TestHaiku55PreservesConversation(t *testing.T) {
	c, _ := NewCompleter("http://localhost", "claude-haiku-5-5")
	history := []provider.Message{
		provider.SystemMessage("Stable instructions"),
		provider.UserMessage("Plan"),
		{Role: provider.MessageRoleAssistant, Content: []provider.Content{
			provider.ReasoningContent(provider.Reasoning{Signature: "signed-state"}),
			provider.TextContent("Plan ready"),
		}},
		{Role: provider.MessageRoleSystem, Content: []provider.Content{provider.InstructionsContent(provider.Instructions{Text: "Be concise", Scope: provider.InstructionScopeTurn})}},
		{Content: []provider.Content{provider.ConfigurationUpdateContent(provider.ConfigurationUpdate{ReasoningEffort: provider.EffortLow})}},
		provider.UserMessage("Summarize"),
	}
	body := requestBody(t, c, history, &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Effort: provider.EffortHigh, Context: provider.ReasoningContextCurrentTurn}})
	messages := body["messages"].([]any)
	if len(messages) != 5 || len(body["system"].([]any)) != 1 {
		t.Fatalf("changed conversation structure: %v", body)
	}
	blocks := messages[1].(map[string]any)["content"].([]any)
	thinking := blocks[0].(map[string]any)
	if thinking["type"] != "thinking" || thinking["thinking"] != "" || thinking["signature"] != "signed-state" {
		t.Fatalf("lost signature-only thinking: %v", thinking)
	}
	reminder := messages[2].(map[string]any)
	update := messages[3].(map[string]any)
	if reminder["role"] != "system" || reminder["clear_at"] != "next_user_message" || update["output_config"].(map[string]any)["effort"] != "low" {
		t.Fatalf("lost positional updates: %v", messages)
	}
	if body["output_config"].(map[string]any)["effort"] != "high" || body["context_management"] == nil {
		t.Fatalf("lost prefix effort or default-thinking retention: %v", body)
	}
}
