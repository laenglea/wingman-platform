package provider

import "testing"

func TestCacheUsageSnapshots(t *testing.T) {
	var acc CompletionAccumulator
	for _, snapshot := range []struct {
		total       int
		details     *CacheWriteTokensDetails
		want        int
		wantDetails *CacheWriteTokensDetails
	}{
		{100, &CacheWriteTokensDetails{StandardTokens: 40, ExtendedTokens: 60}, 100, &CacheWriteTokensDetails{StandardTokens: 40, ExtendedTokens: 60}},
		{80, &CacheWriteTokensDetails{StandardTokens: 80}, 100, &CacheWriteTokensDetails{StandardTokens: 40, ExtendedTokens: 60}},
		{100, nil, 100, &CacheWriteTokensDetails{StandardTokens: 40, ExtendedTokens: 60}},
		{120, nil, 120, nil},
		{120, &CacheWriteTokensDetails{StandardTokens: 20, ExtendedTokens: 100}, 120, &CacheWriteTokensDetails{StandardTokens: 20, ExtendedTokens: 100}},
	} {
		acc.Add(Completion{Usage: &Usage{CacheCreationInputTokens: snapshot.total, CacheWriteTokensDetails: snapshot.details}})
		u := acc.Result().Usage
		if u.CacheCreationInputTokens != snapshot.want || (u.CacheWriteTokensDetails == nil) != (snapshot.wantDetails == nil) || snapshot.wantDetails != nil && *u.CacheWriteTokensDetails != *snapshot.wantDetails {
			t.Fatalf("cache usage = %+v (%+v), want %d (%+v)", u, u.CacheWriteTokensDetails, snapshot.want, snapshot.wantDetails)
		}
	}
}
