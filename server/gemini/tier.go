package gemini

import "github.com/adrianliechti/wingman/pkg/provider"

func parseTier(tier string) provider.Tier {
	switch tier {
	case "standard":
		return provider.TierStandard
	case "flex":
		return provider.TierFlex
	case "priority":
		return provider.TierPriority
	default:
		return ""
	}
}

func serviceTier(tier provider.Tier) string {
	switch tier {
	case provider.TierStandard:
		return "standard"
	case provider.TierFlex:
		return "flex"
	case provider.TierPriority:
		return "priority"
	default:
		return ""
	}
}
