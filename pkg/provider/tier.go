package provider

// Tier selects a backend's processing tier. Availability depends on
// the provider, model, region, and account. An empty value keeps its default.
type Tier string

const (
	TierDefault  Tier = "default"
	TierFlex     Tier = "flex"
	TierPriority Tier = "priority"
)

// ParseTier treats auto and unknown preferences as an omitted value so clients
// can share a configuration across backends with different tier support.
func ParseTier(tier string) Tier {
	switch value := Tier(tier); value {
	case TierDefault, TierFlex, TierPriority:
		return value
	default:
		return ""
	}
}

// NormalizeTier translates the native standard-tier spelling without
// conflating speed modes, priority capacity, or the request-only auto value.
func NormalizeTier(tier string) Tier {
	if tier == "standard" {
		return TierDefault
	}
	return Tier(tier)
}
