package signatures

import (
	"context"
	"iter"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type captureCompleter struct {
	messages []provider.Message
	options  *provider.CompleteOptions

	reply *provider.Completion
}

func (c *captureCompleter) Complete(_ context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		c.messages = messages
		c.options = options
		yield(c.reply, nil)
	}
}

func TestCompleterStripsSignatures(t *testing.T) {
	inner := &captureCompleter{
		reply: &provider.Completion{
			Message: &provider.Message{
				Role: provider.MessageRoleAssistant,
				Content: []provider.Content{
					provider.ReasoningContent(provider.Reasoning{ID: "rs_2", Summary: "thinking", Signature: "fresh-blob"}),
					provider.TextContent("answer"),
				},
			},
		},
	}

	messages := []provider.Message{
		provider.UserMessage("hi"),
		{
			Role: provider.MessageRoleAssistant,
			Content: []provider.Content{
				provider.ReasoningContent(provider.Reasoning{ID: "rs_1", Signature: "foreign-blob"}),
				provider.ReasoningContent(provider.Reasoning{ID: "rs_r", Signature: "redacted-blob", Redacted: true}),
				provider.ReasoningContent(provider.Reasoning{ID: "rs_t", Text: "kept thought", Signature: "signed"}),
				provider.CompactionContent(provider.Compaction{ID: "cp_1", Signature: "compaction-blob"}),
				provider.CompactionContent(provider.Compaction{ID: "cp_2", Content: "summary text", Signature: "signed"}),
				provider.TextContent("hello"),
			},
		},
	}

	options := &provider.CompleteOptions{
		ReasoningOptions: &provider.ReasoningOptions{IncludeSignature: true, IncludeSummary: true},
	}

	var got *provider.Completion
	for completion, err := range FromCompleter(inner).Complete(context.Background(), messages, options) {
		if err != nil {
			t.Fatal(err)
		}
		got = completion
	}

	assistant := inner.messages[1]
	if len(assistant.Content) != 3 {
		t.Fatalf("assistant contents: %+v", assistant.Content)
	}
	if r := assistant.Content[0].Reasoning; r == nil || r.Signature != "" || r.Text != "kept thought" {
		t.Fatalf("reasoning content: %+v", assistant.Content[0].Reasoning)
	}
	if c := assistant.Content[1].Compaction; c == nil || c.Signature != "" || c.Content != "summary text" {
		t.Fatalf("compaction content: %+v", assistant.Content[1].Compaction)
	}
	if assistant.Content[2].Text != "hello" {
		t.Fatalf("text content: %+v", assistant.Content[2])
	}

	if inner.options.ReasoningOptions.IncludeSignature {
		t.Fatal("IncludeSignature must be cleared")
	}
	if !inner.options.ReasoningOptions.IncludeSummary {
		t.Fatal("IncludeSummary must be preserved")
	}
	if !options.ReasoningOptions.IncludeSignature {
		t.Fatal("caller options must not be mutated")
	}
	if messages[1].Content[0].Reasoning.Signature != "foreign-blob" {
		t.Fatal("caller messages must not be mutated")
	}

	if len(got.Message.Content) != 2 {
		t.Fatalf("reply contents: %+v", got.Message.Content)
	}
	if r := got.Message.Content[0].Reasoning; r == nil || r.Signature != "" || r.Summary != "thinking" {
		t.Fatalf("reply reasoning: %+v", got.Message.Content[0].Reasoning)
	}
}
