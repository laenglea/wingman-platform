package anthropic

import "github.com/adrianliechti/wingman/pkg/provider"

func tierFromSpeed(speed string) provider.Tier {
	switch speed {
	case "standard":
		return provider.TierStandard
	case "fast":
		return provider.TierPriority
	default:
		return ""
	}
}

func speedFromTier(tier provider.Tier) string {
	switch tier {
	case provider.TierStandard:
		return "standard"
	case provider.TierPriority:
		return "fast"
	default:
		return ""
	}
}
