package chat

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestStreamingAccumulatorUsageAlwaysIncludesPromptTokensDetails(t *testing.T) {
	var usage *Usage
	acc := NewStreamingAccumulator("model", func(event StreamEvent) error {
		if event.Type == StreamEventUsage && event.Chunk != nil {
			usage = event.Chunk.Usage
		}
		return nil
	})

	if err := acc.Add(provider.Completion{
		Usage: &provider.Usage{InputTokens: 10, OutputTokens: 5},
	}); err != nil {
		t.Fatalf("add usage: %v", err)
	}
	if err := acc.Complete(true); err != nil {
		t.Fatalf("complete stream: %v", err)
	}

	if usage == nil || usage.PromptTokensDetails == nil {
		t.Fatal("expected prompt_tokens_details to be present even with no cache hit")
	}
	if usage.PromptTokensDetails.CachedTokens != 0 {
		t.Fatalf("expected cached tokens 0, got %d", usage.PromptTokensDetails.CachedTokens)
	}
}

func TestStreamingAccumulatorUsageIncludesCachedTokens(t *testing.T) {
	var usage *Usage
	acc := NewStreamingAccumulator("model", func(event StreamEvent) error {
		if event.Type == StreamEventUsage && event.Chunk != nil {
			usage = event.Chunk.Usage
		}
		return nil
	})

	err := acc.Add(provider.Completion{
		Usage: &provider.Usage{
			InputTokens:          100,
			OutputTokens:         20,
			CacheReadInputTokens: 80,
		},
	})
	if err != nil {
		t.Fatalf("add usage: %v", err)
	}

	err = acc.Complete(true)
	if err != nil {
		t.Fatalf("complete stream: %v", err)
	}

	if usage == nil {
		t.Fatal("expected usage chunk")
	}
	if usage.PromptTokensDetails == nil {
		t.Fatal("expected prompt token details")
	}
	if usage.PromptTokensDetails.CachedTokens != 80 {
		t.Fatalf("expected cached tokens 80, got %d", usage.PromptTokensDetails.CachedTokens)
	}
}
