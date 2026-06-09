package chat

import (
	"testing"
)

func intPtr(v int) *int { return &v }

// TestToCompleteOptions_MaxTokensFallback verifies the deprecated max_tokens
// parameter is honored when max_completion_tokens is absent.
func TestToCompleteOptions_MaxTokensFallback(t *testing.T) {
	options := toCompleteOptions(ChatCompletionRequest{MaxTokens: intPtr(512)}, nil)

	if options.MaxTokens == nil || *options.MaxTokens != 512 {
		t.Fatalf("expected max tokens 512, got %v", options.MaxTokens)
	}
}

func TestToCompleteOptions_MaxCompletionTokensPrecedence(t *testing.T) {
	options := toCompleteOptions(ChatCompletionRequest{
		MaxCompletionTokens: intPtr(1024),
		MaxTokens:           intPtr(512),
	}, nil)

	if options.MaxTokens == nil || *options.MaxTokens != 1024 {
		t.Fatalf("expected max_completion_tokens to win, got %v", options.MaxTokens)
	}
}

func TestToCompleteOptions_Stop(t *testing.T) {
	options := toCompleteOptions(ChatCompletionRequest{Stop: "END"}, nil)
	if len(options.Stop) != 1 || options.Stop[0] != "END" {
		t.Fatalf("string stop: got %v", options.Stop)
	}

	options = toCompleteOptions(ChatCompletionRequest{Stop: []any{"A", "B"}}, nil)
	if len(options.Stop) != 2 || options.Stop[0] != "A" || options.Stop[1] != "B" {
		t.Fatalf("array stop: got %v", options.Stop)
	}
}
