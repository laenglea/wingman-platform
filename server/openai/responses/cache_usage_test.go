package responses

import (
	"github.com/adrianliechti/wingman/pkg/provider"
	"testing"
)

func TestCacheUsageSnapshots(t *testing.T) {
	var usage *provider.Usage
	mergeUsage(&usage, &provider.Usage{CacheCreationInputTokens: 100, CacheWriteTokensDetails: &provider.CacheWriteTokensDetails{ExtendedTokens: 100}})
	mergeUsage(&usage, &provider.Usage{CacheCreationInputTokens: 80, CacheWriteTokensDetails: &provider.CacheWriteTokensDetails{StandardTokens: 80}})
	if usage.CacheCreationInputTokens != 100 || usage.CacheWriteTokensDetails.ExtendedTokens != 100 || usage.CacheWriteTokensDetails.StandardTokens != 0 {
		t.Fatalf("inconsistent cache usage: %+v", usage)
	}
	mergeUsage(&usage, &provider.Usage{CacheCreationInputTokens: 120})
	if usage.CacheWriteTokensDetails != nil {
		t.Fatal("kept stale cache breakdown")
	}
}
