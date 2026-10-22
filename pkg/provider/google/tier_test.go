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
		{"", ""}, {provider.Tier("auto"), ""}, {provider.TierStandard, "standard"},
		{provider.TierFlex, "flex"}, {provider.TierPriority, "priority"}, {"unknown", ""},
		{"default", ""}, {"fast", ""}, {"ultrafast", ""},
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
	for _, test := range []struct {
		native string
		want   provider.Tier
	}{
		{"standard", provider.TierStandard}, {"flex", provider.TierFlex}, {"priority", provider.TierPriority},
		{"", provider.TierPriority}, {"unknown", provider.TierPriority}, {"default", provider.TierPriority},
	} {
		t.Run(test.native, func(t *testing.T) {
			deltas, err := completeAll(t, interactionEvents(
				`{"event_type":"interaction.created","interaction":{"id":"r1","model":"gemini-3.8-flash","status":"in_progress","service_tier":"priority"}}`,
				`{"event_type":"interaction.completed","interaction":{"id":"r1","status":"completed","service_tier":"`+test.native+`"}}`,
			), &provider.CompleteOptions{Tier: provider.TierPriority})
			if err != nil {
				t.Fatal(err)
			}
			if got := accumulated(deltas).Tier; got != test.want {
				t.Fatalf("effective tier = %q, want %q", got, test.want)
			}
		})
	}
}
