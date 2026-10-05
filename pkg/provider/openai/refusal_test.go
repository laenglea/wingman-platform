package openai

import (
	"github.com/adrianliechti/wingman/pkg/provider"
	"testing"
)

func TestRefusalReplayedInAssistantHistory(t *testing.T) {
	messages := []provider.Message{{Role: provider.MessageRoleAssistant, Content: []provider.Content{
		{MessageID: "msg_refusal", Text: "partial"}, {MessageID: "msg_refusal", Refusal: "Cannot help."},
	}}}
	responder, _ := NewResponder("https://api.openai.com/v1", "test")
	body := responsesRequestBody(t, responder, messages, &provider.CompleteOptions{})
	input := body["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("replayed %d items, want one assistant message", len(input))
	}
	content := input[0].(map[string]any)["content"].([]any)
	if len(content) != 2 || content[1].(map[string]any)["type"] != "refusal" || content[1].(map[string]any)["refusal"] != "Cannot help." {
		t.Fatalf("refusal history lost: %+v", input)
	}
	chat := convertedMessages(t, messages)
	if len(chat) != 1 || chat[0]["refusal"] != "Cannot help." {
		t.Fatalf("chat refusal history lost: %+v", chat)
	}
}
