package google

import (
	"github.com/adrianliechti/wingman/pkg/provider"
	"google.golang.org/genai/interactions/models/interactions"
)

func (s *interactionState) setTier(tier *interactions.ServiceTier) {
	if tier != nil && *tier != "" {
		s.tier = provider.NormalizeTier(string(*tier))
	}
}
