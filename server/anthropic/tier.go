package anthropic

import "github.com/adrianliechti/wingman/pkg/provider"

func speedFromTier(tier provider.Tier) string {
	switch tier {
	case provider.TierDefault:
		return "standard"
	case provider.TierPriority:
		return "fast"
	default:
		return ""
	}
}
