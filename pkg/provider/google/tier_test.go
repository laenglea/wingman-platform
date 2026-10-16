package google

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestTierMapping(t *testing.T) {
	for _, test := range []struct {
		tier   provider.Tier
		native string
	}{
		{"", ""}, {provider.Tier("auto"), ""}, {provider.TierDefault, "standard"},
		{provider.TierFlex, "flex"}, {provider.TierPriority, "priority"}, {"unknown", ""},
	} {
		req, err := convertInteraction("gemini-3.8-flash", nil, &provider.CompleteOptions{Tier: test.tier})
		if err != nil {
			t.Fatal(err)
		}
		got := string(value(req.ServiceTier))
		if got != test.native {
			t.Errorf("%s: native tier = %q, want %q", test.tier, got, test.native)
		}
	}
}

func TestTierFromInteraction(t *testing.T) {
	deltas, err := completeAll(t, interactionEvents(
		`{"event_type":"interaction.created","interaction":{"id":"r1","model":"gemini-3.8-flash","status":"in_progress","service_tier":"priority"}}`,
		`{"event_type":"interaction.completed","interaction":{"id":"r1","status":"completed","service_tier":"standard"}}`,
	), &provider.CompleteOptions{Tier: provider.TierPriority})
	if err != nil {
		t.Fatal(err)
	}
	if got := accumulated(deltas).Tier; got != provider.TierDefault {
		t.Fatalf("effective tier = %q, want default", got)
	}
}
