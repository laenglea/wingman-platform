package anthropic

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// TestToCompleteOptions_OutputConfigFormat verifies the canonical
// output_config.format parameter maps to a schema (the deprecated top-level
// output_format is the fallback).
func TestToCompleteOptions_OutputConfigFormat(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}

	options, err := toCompleteOptions(MessageRequest{
		OutputConfig: &OutputConfig{
			Format: &OutputFormat{Type: "json_schema", Schema: schema},
		},
	})
	if err != nil {
		t.Fatalf("toCompleteOptions: %v", err)
	}

	if options.Schema == nil {
		t.Fatal("expected schema from output_config.format")
	}
	if options.Schema.Name != "response" {
		t.Errorf("schema name: got %q, want default \"response\"", options.Schema.Name)
	}
	if options.Schema.Properties == nil {
		t.Error("expected schema properties")
	}
}

func TestToCompleteOptions_OutputConfigFormatPrecedence(t *testing.T) {
	options, err := toCompleteOptions(MessageRequest{
		OutputFormat: &OutputFormat{Type: "json_schema", Name: "legacy", Schema: map[string]any{"type": "object"}},
		OutputConfig: &OutputConfig{
			Format: &OutputFormat{Type: "json_schema", Name: "canonical", Schema: map[string]any{"type": "object"}},
		},
	})
	if err != nil {
		t.Fatalf("toCompleteOptions: %v", err)
	}

	if options.Schema == nil || options.Schema.Name != "canonical" {
		t.Fatalf("expected output_config.format to win, got %+v", options.Schema)
	}
}

func TestToCompleteOptions_LegacyOutputFormat(t *testing.T) {
	options, err := toCompleteOptions(MessageRequest{
		OutputFormat: &OutputFormat{Type: "json_schema", Name: "legacy", Schema: map[string]any{"type": "object"}},
	})
	if err != nil {
		t.Fatalf("toCompleteOptions: %v", err)
	}

	if options.Schema == nil || options.Schema.Name != "legacy" {
		t.Fatalf("expected legacy output_format schema, got %+v", options.Schema)
	}
}

func TestToCompleteOptions_Thinking(t *testing.T) {
	cases := []struct {
		name       string
		req        MessageRequest
		wantType   provider.ReasoningType
		wantEffort provider.Effort
		wantNil    bool
	}{
		{
			name:     "adaptive",
			req:      MessageRequest{Thinking: &ThinkingConfig{Type: "adaptive"}},
			wantType: provider.ReasoningTypeAdaptive,
		},
		{
			name:       "enabled with budget derives effort",
			req:        MessageRequest{Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 2048}},
			wantType:   provider.ReasoningTypeAdaptive,
			wantEffort: provider.EffortLow,
		},
		{
			name:     "disabled",
			req:      MessageRequest{Thinking: &ThinkingConfig{Type: "disabled"}},
			wantType: provider.ReasoningTypeDisabled,
		},
		{
			name:       "effort only",
			req:        MessageRequest{OutputConfig: &OutputConfig{Effort: "xhigh"}},
			wantEffort: provider.EffortXHigh,
		},
		{
			name:    "no thinking",
			req:     MessageRequest{},
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options, err := toCompleteOptions(tc.req)
			if err != nil {
				t.Fatalf("toCompleteOptions: %v", err)
			}

			if tc.wantNil {
				if options.ReasoningOptions != nil {
					t.Fatalf("expected nil reasoning options, got %+v", options.ReasoningOptions)
				}
				return
			}

			if options.ReasoningOptions == nil {
				t.Fatal("expected reasoning options")
			}
			if options.ReasoningOptions.Type != tc.wantType {
				t.Errorf("type: got %q, want %q", options.ReasoningOptions.Type, tc.wantType)
			}
			if options.ReasoningOptions.Effort != tc.wantEffort {
				t.Errorf("effort: got %q, want %q", options.ReasoningOptions.Effort, tc.wantEffort)
			}
		})
	}
}

// TestToCompleteOptions_CompactionWithoutTrigger verifies a compaction edit
// without an explicit trigger still enables compaction (upstream default).
func TestToCompleteOptions_CompactionWithoutTrigger(t *testing.T) {
	options, err := toCompleteOptions(MessageRequest{
		ContextManagement: &ContextManagement{
			Edits: []ContextManagementEdit{{Type: "compact_20260112"}},
		},
	})
	if err != nil {
		t.Fatalf("toCompleteOptions: %v", err)
	}

	if options.CompactionOptions == nil {
		t.Fatal("expected compaction options")
	}
	if options.CompactionOptions.Threshold != 0 {
		t.Errorf("threshold: got %d, want 0 (upstream default)", options.CompactionOptions.Threshold)
	}
}

func TestToCompleteOptions_ToolChoice(t *testing.T) {
	options, err := toCompleteOptions(MessageRequest{
		Tools:      []ToolParam{{Name: "get_weather", InputSchema: map[string]any{"type": "object"}}},
		ToolChoice: &ToolChoice{Type: "tool", Name: "get_weather", DisableParallelToolUse: true},
	})
	if err != nil {
		t.Fatalf("toCompleteOptions: %v", err)
	}

	if options.ToolOptions == nil {
		t.Fatal("expected tool options")
	}
	if options.ToolOptions.Choice != provider.ToolChoiceAny {
		t.Errorf("choice: got %q", options.ToolOptions.Choice)
	}
	if len(options.ToolOptions.Allowed) != 1 || options.ToolOptions.Allowed[0] != "get_weather" {
		t.Errorf("allowed: got %v", options.ToolOptions.Allowed)
	}
	if !options.ToolOptions.DisableParallelToolCalls {
		t.Error("expected parallel tool calls disabled")
	}
}
