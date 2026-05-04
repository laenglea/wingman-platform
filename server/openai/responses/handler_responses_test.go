package responses

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestResponseUsageIncludesCachedTokens(t *testing.T) {
	usage := responseUsage(&provider.Usage{
		InputTokens:          100,
		OutputTokens:         20,
		CacheReadInputTokens: 80,
	})

	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.InputTokensDetails == nil {
		t.Fatal("expected input token details")
	}
	if usage.InputTokensDetails.CachedTokens != 80 {
		t.Fatalf("expected cached tokens 80, got %d", usage.InputTokensDetails.CachedTokens)
	}
	if usage.TotalTokens != 120 {
		t.Fatalf("expected total tokens 120, got %d", usage.TotalTokens)
	}
}
