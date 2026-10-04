package claude

import "github.com/adrianliechti/wingman/pkg/provider"

// Thinking is the resolved Claude thinking configuration for a request.
type Thinking struct {
	Enabled    bool
	Disabled   bool
	Summarized bool
	Updates    bool
	Effort     string
}

// OutputEffort maps shared effort levels to Claude's effort values.
func OutputEffort(e provider.Effort) string {
	switch e {
	case provider.EffortMinimal, provider.EffortLow:
		return "low"
	case provider.EffortMedium:
		return "medium"
	case provider.EffortHigh:
		return "high"
	case provider.EffortXHigh:
		return "xhigh"
	case provider.EffortMax:
		return "max"
	}
	return ""
}

// ResolveThinking maps request reasoning to Claude's thinking policy. Model
// eligibility and native request encoding remain in the adapters.
func ResolveThinking(model string, r *provider.ReasoningOptions, forced bool) Thinking {
	var t Thinking

	if r != nil {
		t.Effort = OutputEffort(r.Effort)
		t.Summarized = r.IncludeSummary
		t.Updates = r.IncludeUpdates && !r.IncludeSummary && MatchesModel(model, ProgressUpdateModels)

		switch r.Type {
		case provider.ReasoningTypeAdaptive:
			t.Enabled = true
		case provider.ReasoningTypeDisabled:
			t.Disabled = true
		}
	}

	// A forced tool call is incompatible with thinking. Replayed history
	// never is: unsigned reasoning is not sent, and adaptive thinking accepts
	// a tool turn without a thinking block. Keeping the thinking parameter
	// stable across turns also keeps the prompt cache prefix stable.
	if forced {
		t.Enabled = false
		t.Disabled = true
	}

	if MatchesModel(model, AlwaysThinkingModels) {
		t.Disabled = false
	}

	// Display is only sent with an explicit adaptive config; without one, a
	// model that thinks by default uses its default display, "omitted".
	if !t.Enabled && !t.Disabled && !forced && (t.Summarized || t.Updates) &&
		(MatchesModel(model, AlwaysThinkingModels) || MatchesModel(model, DefaultThinkingModels)) {
		t.Enabled = true
	}

	if t.Disabled && MatchesModel(model, DisabledThinkingEffortCapModels) && (t.Effort == "xhigh" || t.Effort == "max") {
		t.Effort = "high"
	}

	return t
}
