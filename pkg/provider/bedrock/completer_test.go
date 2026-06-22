package bedrock

import (
	"encoding/base64"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

var testSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title": map[string]any{"type": "string"},
	},
	"required":             []string{"title"},
	"additionalProperties": false,
}

func TestConvertConverseInputUsesForcedToolForSchema(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-opus-4-8-v1:0"}}

	req, err := c.convertConverseInput([]provider.Message{
		provider.UserMessage("Return JSON."),
	}, &provider.CompleteOptions{
		Schema: &provider.Schema{
			Name:       "classify_chat",
			Properties: testSchema,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.ToolConfig == nil {
		t.Fatal("expected tool config for schema mode")
	}

	choice, ok := req.ToolConfig.ToolChoice.(*types.ToolChoiceMemberTool)
	if !ok {
		t.Fatalf("expected forced tool choice, got %T", req.ToolConfig.ToolChoice)
	}
	if got := *choice.Value.Name; got != "classify_chat" {
		t.Fatalf("expected forced tool %q, got %q", "classify_chat", got)
	}

	var found bool
	for _, tool := range req.ToolConfig.Tools {
		spec, ok := tool.(*types.ToolMemberToolSpec)
		if ok && *spec.Value.Name == "classify_chat" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected schema tool in tool config")
	}
}

// TestConvertAssistantContent_Reasoning verifies signed thinking maps to a
// reasoning text block and redacted thinking to a redacted content block with
// the blob decoded from base64.
func TestConvertAssistantContent_Reasoning(t *testing.T) {
	blob := []byte{0x01, 0x02, 0x03, 0xff}

	content, err := convertAssistantContent(provider.Message{
		Role: provider.MessageRoleAssistant,
		Content: []provider.Content{
			provider.ReasoningContent(provider.Reasoning{Text: "step", Signature: "SIG"}),
			provider.ReasoningContent(provider.Reasoning{Signature: base64.StdEncoding.EncodeToString(blob), Redacted: true}),
			provider.TextContent("answer"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(content) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(content))
	}

	first, ok := content[0].(*types.ContentBlockMemberReasoningContent)
	if !ok {
		t.Fatalf("block 0: got %T", content[0])
	}
	text, ok := first.Value.(*types.ReasoningContentBlockMemberReasoningText)
	if !ok {
		t.Fatalf("block 0: got %T", first.Value)
	}
	if *text.Value.Text != "step" || *text.Value.Signature != "SIG" {
		t.Errorf("block 0: %+v", text.Value)
	}

	second, ok := content[1].(*types.ContentBlockMemberReasoningContent)
	if !ok {
		t.Fatalf("block 1: got %T", content[1])
	}
	redacted, ok := second.Value.(*types.ReasoningContentBlockMemberRedactedContent)
	if !ok {
		t.Fatalf("block 1: got %T", second.Value)
	}
	if string(redacted.Value) != string(blob) {
		t.Errorf("block 1: got %v, want %v", redacted.Value, blob)
	}

	if _, ok := content[2].(*types.ContentBlockMemberText); !ok {
		t.Fatalf("block 2: got %T", content[2])
	}
}

// TestToUsage_CacheInclusiveInputTokens verifies the intermediate Usage uses a
// cache-inclusive InputTokens total. Bedrock reports InputTokens as only the
// fresh (non-cached) tokens, with cache read/write counted separately, so the
// mapping must fold both back into InputTokens while still exposing the cached
// subset in the cache fields.
func TestToUsage_CacheInclusiveInputTokens(t *testing.T) {
	usage := toUsage(&types.TokenUsage{
		InputTokens:           aws.Int32(10),
		OutputTokens:          aws.Int32(7),
		CacheReadInputTokens:  aws.Int32(40),
		CacheWriteInputTokens: aws.Int32(50),
	})

	if usage == nil {
		t.Fatal("expected usage")
	}

	if usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (10 fresh + 40 read + 50 write)", usage.InputTokens)
	}
	if usage.OutputTokens != 7 {
		t.Errorf("OutputTokens = %d, want 7", usage.OutputTokens)
	}
	if usage.CacheReadInputTokens != 40 {
		t.Errorf("CacheReadInputTokens = %d, want 40", usage.CacheReadInputTokens)
	}
	if usage.CacheCreationInputTokens != 50 {
		t.Errorf("CacheCreationInputTokens = %d, want 50", usage.CacheCreationInputTokens)
	}

	// Cache fields must always be a subset of the inclusive input total.
	if usage.CacheReadInputTokens+usage.CacheCreationInputTokens > usage.InputTokens {
		t.Errorf("cache tokens (%d+%d) exceed InputTokens (%d)",
			usage.CacheReadInputTokens, usage.CacheCreationInputTokens, usage.InputTokens)
	}
}

func TestToUsage_NilReturnsNil(t *testing.T) {
	if usage := toUsage(nil); usage != nil {
		t.Fatalf("expected nil usage, got %+v", usage)
	}
}
