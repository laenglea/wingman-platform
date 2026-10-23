package bedrock

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestTierMapping(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-opus-5-5"}}
	for _, test := range []struct {
		tier provider.Tier
		wire string
	}{
		{"", ""}, {"auto", ""}, {provider.TierStandard, "default"},
		{provider.TierFlex, "flex"}, {provider.TierPriority, "priority"}, {"unknown", ""},
		{"default", ""}, {"fast", ""}, {"ultrafast", ""},
	} {
		tier := test.tier
		req, err := c.convertConverseInput([]provider.Message{provider.UserMessage("hi")}, &provider.CompleteOptions{Tier: tier})
		if err != nil {
			t.Fatal(err)
		}
		want := test.wire
		if want == "" {
			if req.ServiceTier != nil {
				t.Fatalf("%q should keep backend default", tier)
			}
		} else if req.ServiceTier == nil || string(req.ServiceTier.Type) != want {
			t.Fatalf("%q: native tier = %+v", tier, req.ServiceTier)
		}
	}
}

func TestTierFromMetadata(t *testing.T) {
	for _, test := range []struct {
		native string
		want   provider.Tier
	}{
		{"default", provider.TierStandard}, {"flex", provider.TierFlex}, {"priority", provider.TierPriority},
		{"standard", ""}, {"fast", ""}, {"unknown", ""},
	} {
		t.Run(test.native, func(t *testing.T) {
			c := scriptedCompleter(t,
				[2]string{"messageStart", `{"role":"assistant"}`},
				[2]string{"messageStop", `{"stopReason":"end_turn"}`},
				[2]string{"metadata", `{"usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5},"metrics":{"latencyMs":1},"serviceTier":{"type":"` + test.native + `"}}`},
			)
			var acc provider.CompletionAccumulator
			for delta, err := range c.Complete(t.Context(), []provider.Message{provider.UserMessage("hi")}, &provider.CompleteOptions{Tier: provider.TierFlex}) {
				if err != nil {
					t.Fatal(err)
				}
				acc.Add(*delta)
			}
			if got := acc.Result().Tier; got != test.want {
				t.Fatalf("effective tier = %q, want %q", got, test.want)
			}
		})
	}
}
