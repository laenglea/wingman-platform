package responses

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestStreamingAccumulatorMergesUsage(t *testing.T) {
	acc := NewStreamingAccumulator(func(StreamEvent) error {
		return nil
	})

	err := acc.Add(provider.Completion{
		Usage: &provider.Usage{
			InputTokens:              100,
			CacheReadInputTokens:     80,
			CacheCreationInputTokens: 20,
		},
	})
	if err != nil {
		t.Fatalf("add first usage: %v", err)
	}

	err = acc.Add(provider.Completion{
		Usage: &provider.Usage{
			OutputTokens:         30,
			CacheReadInputTokens: 70,
		},
	})
	if err != nil {
		t.Fatalf("add second usage: %v", err)
	}

	usage := acc.Result().Usage
	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.InputTokens != 100 {
		t.Fatalf("expected input tokens 100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 30 {
		t.Fatalf("expected output tokens 30, got %d", usage.OutputTokens)
	}
	if usage.CacheReadInputTokens != 80 {
		t.Fatalf("expected cache read input tokens 80, got %d", usage.CacheReadInputTokens)
	}
	if usage.CacheCreationInputTokens != 20 {
		t.Fatalf("expected cache creation input tokens 20, got %d", usage.CacheCreationInputTokens)
	}
}
