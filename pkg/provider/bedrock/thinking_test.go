package bedrock

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestResolveThinking(t *testing.T) {
	adaptive := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeAdaptive, Effort: provider.EffortMax, IncludeSummary: true}}
	disabled := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeDisabled, Effort: provider.EffortMax}}
	updates := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeAdaptive, Effort: provider.EffortLow, IncludeUpdates: true}}
	effortOnly := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{Effort: provider.EffortLow}}
	summaryOnly := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{IncludeSummary: true}}
	updatesOnly := &provider.CompleteOptions{ReasoningOptions: &provider.ReasoningOptions{IncludeUpdates: true}}

	cases := []struct {
		name    string
		model   string
		options *provider.CompleteOptions
		forced  bool
		want    thinking
	}{
		{"adaptive", "eu.anthropic.claude-sonnet-4-6", adaptive, false, thinking{Enabled: true, Summarized: true, Effort: "max"}},
		{"effort only leaves thinking alone", "eu.anthropic.claude-sonnet-4-6", effortOnly, false, thinking{Effort: "low"}},
		{"disabled", "eu.anthropic.claude-sonnet-4-6", disabled, false, thinking{Disabled: true, Effort: "max"}},
		{"legacy ignores everything", "eu.anthropic.claude-sonnet-4-5-20250929-v1:0", adaptive, false, thinking{}},
		{"dated sonnet 4 is legacy", "eu.anthropic.claude-sonnet-4-20250514-v1:0", adaptive, false, thinking{}},
		{"forced tool disables", "eu.anthropic.claude-sonnet-4-6", adaptive, true, thinking{Disabled: true, Summarized: true, Effort: "max"}},
		{"disabled effort is capped", "eu.anthropic.claude-opus-5", disabled, false, thinking{Disabled: true, Effort: "high"}},
		{"opus 5.5 cannot disable", "anthropic.claude-opus-5-5", disabled, false, thinking{Effort: "max"}},
		{"updates on a progress-update model", "eu.anthropic.claude-opus-5-5", updates, false, thinking{Enabled: true, Updates: true, Effort: "low"}},
		{"updates elsewhere are omitted", "eu.anthropic.claude-sonnet-4-6", updates, false, thinking{Enabled: true, Effort: "low"}},
		{"sonnet 5.5 disabled effort is capped", "anthropic.claude-sonnet-5-5", disabled, false, thinking{Disabled: true, Effort: "high"}},
		{"summary sets display on a default-thinking model", "eu.anthropic.claude-sonnet-5", summaryOnly, false, thinking{Enabled: true, Summarized: true}},
		{"updates set display on a default-thinking model", "anthropic.claude-opus-5-5", updatesOnly, false, thinking{Enabled: true, Updates: true}},
		{"summary leaves an opt-in model off", "eu.anthropic.claude-opus-4-8", summaryOnly, false, thinking{Summarized: true}},
		{"forced tool keeps default display", "anthropic.claude-fable-5", summaryOnly, true, thinking{Summarized: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Completer{Config: &Config{model: tc.model}}

			if got := c.resolveThinking(tc.options, tc.forced); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestProgressNotes verifies thinking text is labeled as a summary only when
// the request can return nothing but progress notes.
func TestProgressNotes(t *testing.T) {
	for _, tc := range []struct {
		model string
		t     thinking
		want  bool
	}{
		{"anthropic.claude-opus-5-5", thinking{Enabled: true, Updates: true}, true},
		{"anthropic.claude-sonnet-5-5", thinking{Disabled: true}, true},
		{"anthropic.claude-sonnet-5", thinking{Disabled: true}, false},
		{"anthropic.claude-opus-5-5", thinking{Enabled: true, Summarized: true}, false},
	} {
		c := &Completer{Config: &Config{model: tc.model}}
		if got := c.progressNotes(tc.t); got != tc.want {
			t.Errorf("%s %+v: got %v, want %v", tc.model, tc.t, got, tc.want)
		}
	}
}
