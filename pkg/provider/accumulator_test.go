package provider

import "testing"

func TestCompletionAccumulatorPreservesCacheUsage(t *testing.T) {
	acc := CompletionAccumulator{}

	acc.Add(Completion{
		Usage: &Usage{
			InputTokens:              12,
			OutputTokens:             3,
			CacheReadInputTokens:     40,
			CacheCreationInputTokens: 50,
		},
	})
	acc.Add(Completion{
		Usage: &Usage{
			InputTokens:              10,
			OutputTokens:             5,
			CacheReadInputTokens:     35,
			CacheCreationInputTokens: 45,
		},
	})

	usage := acc.Result().Usage
	if usage == nil {
		t.Fatal("expected usage")
	}

	if usage.InputTokens != 12 {
		t.Fatalf("expected input tokens 12, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 5 {
		t.Fatalf("expected output tokens 5, got %d", usage.OutputTokens)
	}
	if usage.CacheReadInputTokens != 40 {
		t.Fatalf("expected cache read input tokens 40, got %d", usage.CacheReadInputTokens)
	}
	if usage.CacheCreationInputTokens != 50 {
		t.Fatalf("expected cache creation input tokens 50, got %d", usage.CacheCreationInputTokens)
	}
}
