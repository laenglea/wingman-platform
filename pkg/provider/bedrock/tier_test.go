package bedrock

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestTierMapping(t *testing.T) {
	c := &Completer{Config: &Config{model: "anthropic.claude-opus-5-5"}}
	for _, tier := range []provider.Tier{"", provider.Tier("auto"), provider.TierDefault, provider.TierFlex, provider.TierPriority, "unknown"} {
		req, err := c.convertConverseInput([]provider.Message{provider.UserMessage("hi")}, &provider.CompleteOptions{Tier: tier})
		if err != nil {
			t.Fatal(err)
		}
		want := provider.ParseTier(string(tier))
		if want == "" {
			if req.ServiceTier != nil {
				t.Fatalf("%q should keep backend default", tier)
			}
		} else if req.ServiceTier == nil || string(req.ServiceTier.Type) != string(want) {
			t.Fatalf("%q: native tier = %+v", tier, req.ServiceTier)
		}
	}
}

func TestTierFromMetadata(t *testing.T) {
	c := scriptedCompleter(t,
		[2]string{"messageStart", `{"role":"assistant"}`},
		[2]string{"messageStop", `{"stopReason":"end_turn"}`},
		[2]string{"metadata", `{"usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5},"metrics":{"latencyMs":1},"serviceTier":{"type":"flex"}}`},
	)
	var acc provider.CompletionAccumulator
	for delta, err := range c.Complete(t.Context(), []provider.Message{provider.UserMessage("hi")}, &provider.CompleteOptions{Tier: provider.TierFlex}) {
		if err != nil {
			t.Fatal(err)
		}
		acc.Add(*delta)
	}
	if got := acc.Result().Tier; got != provider.TierFlex {
		t.Fatalf("effective tier = %q, want flex", got)
	}
}
