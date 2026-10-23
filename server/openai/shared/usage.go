package shared

import "github.com/adrianliechti/wingman/pkg/provider"

type CacheWriteTokensDetails struct {
	TTL30mTokens int `json:"ttl_30m_tokens"`
	TTL12hTokens int `json:"ttl_12h_tokens"`
}

func CacheWriteDetails(details *provider.CacheWriteTokensDetails) *CacheWriteTokensDetails {
	if details == nil {
		return nil
	}
	return &CacheWriteTokensDetails{TTL30mTokens: details.StandardTokens, TTL12hTokens: details.ExtendedTokens}
}
