package bedrock

import (
	"fmt"
	"math"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// LegacyModels take neither adaptive thinking nor an effort. Bedrock names
// Sonnet 4 and Opus 4 by date only (claude-sonnet-4-20250514).
var LegacyModels = []string{
	"claude-3",

	"sonnet-4-0",
	"sonnet-4-2025",
	"sonnet-4-5",

	"opus-4-0",
	"opus-4-2025",
	"opus-4-1",
	"opus-4-5",

	"haiku-4-5",
}

// ContextBoundModels predate Claude 4.5 and reject a request whose input plus
// max_tokens exceeds the context window, where later models stop at the
// window instead.
var ContextBoundModels = []string{
	"claude-3",

	"sonnet-4-0",
	"sonnet-4-2025",

	"opus-4-0",
	"opus-4-2025",
	"opus-4-1",
}

// defaultMaxTokens is the output limit sent when the client sets none.
// Converse otherwise applies a far lower model default (4096 on Claude Opus
// 5.5), which thinking and a short preamble can exhaust before the tool call,
// so the turn ends as truncated right after the commentary. Claude models get
// the Anthropic adapter's defaults. Other families keep the Bedrock default,
// as do context-bound models, where a large limit would fail long prompts.
func defaultMaxTokens(model string) int32 {
	switch {
	case !isClaudeModel(model) || matchesModel(model, ContextBoundModels):
		return 0
	case matchesModel(model, LegacyModels):
		return 64000
	default:
		return 128000
	}
}

// NoSamplingModels reject temperature/top_p/top_k outright — the same set as
// on the native Anthropic API.
var NoSamplingModels = []string{
	"fable-5",
	"mythos-5",
	"mythos-preview",

	"opus-4-7",
	"opus-4-8",
	"opus-5",

	"sonnet-5",
}

// DefaultThinkingModels think when the field is omitted but still accept an
// explicit `thinking: {type: "disabled"}` — required by Bedrock for forced
// tool_choice. Fable/Mythos also think by default but reject the disable.
// Patterns match by substring, so "opus-5" also covers Opus 5.5; the
// AlwaysThinkingModels check takes precedence for it.
var DefaultThinkingModels = []string{
	"opus-5",
	"sonnet-5",
}

// AlwaysThinkingModels reject an explicit `thinking: {type: "disabled"}` —
// thinking cannot be turned off on these models.
var AlwaysThinkingModels = []string{
	"fable-5",
	"mythos-5",
	"mythos-preview",

	"opus-5-5",
}

// NoForcedToolChoiceModels reject tool_choice "any" and a named tool. Reject
// such requests rather than weakening the caller's requirement; schema mode
// emulation steers the schema tool with automatic selection instead.
var NoForcedToolChoiceModels = []string{
	"fable-5-1",
	"mythos-5-1",

	"opus-5-5",
	"sonnet-5-5",
}

// BetweenToolsModels reject `thinking: {type: "disabled"}`; their lowest
// setting, `thinking: {type: "between_tools"}`, turns off up-front thinking
// and is sent instead.
var BetweenToolsModels = []string{
	"sonnet-5-5",
}

// progressNotes reports whether returned thinking text holds only the notes
// the model writes between tool calls: display "updates" and between_tools
// return those notes and never reasoning.
func (c *Completer) progressNotes(t thinking) bool {
	return (t.Enabled && t.Updates) || (t.Disabled && matchesModel(c.model, BetweenToolsModels))
}

// thinkingReasoning returns progress notes as a summary, so frontends show
// them without the caller asking for reasoning summaries.
func thinkingReasoning(text string, notes bool) provider.Reasoning {
	if notes {
		return provider.Reasoning{Summary: text}
	}
	return provider.Reasoning{Text: text}
}

// schemaToolInstruction steers schema mode toward the schema tool on models
// that cannot be forced to call it.
const schemaToolInstruction = "Always deliver the final answer by calling this tool, and do not answer in plain text."

// ProgressUpdateModels write progress notes between tool calls, returned
// with `display: "updates"` (beta) while reasoning stays hidden.
var ProgressUpdateModels = []string{
	"fable-5",
	"mythos-5",

	"opus-5-5",
	"sonnet-5-5",
}

// DisabledThinkingEffortCapModels accept `thinking: {type: "disabled"}` (or
// "between_tools") only at effort "high" or below — pairing it with "xhigh"
// or "max" returns a 400.
var DisabledThinkingEffortCapModels = []string{
	"opus-5",
	"sonnet-5-5",
}

// Structured outputs (strict tool use) is supported by the Claude 4.5 and 4.6
// models. Newer ones (4.7, 4.8, 5.x) reject the field outright with
// "tools.N.custom.strict: Extra inputs are not permitted", so this is an
// allowlist: omitting strict degrades to unconstrained tool calls, while
// sending it to a model that lacks support fails the whole request.
var StrictToolModels = []string{
	"sonnet-4-5",
	"sonnet-4-6",

	"opus-4-5",
	"opus-4-6",

	"haiku-4-5",
}

func supportsStrictTools(model string) bool {
	return !isClaudeModel(model) || matchesModel(model, StrictToolModels)
}

// Native JSON-schema output (outputConfig.textFormat) is the same structured
// outputs feature as strict tools, so it follows the Claude allowlist. Other
// model families keep the forced-tool emulation, which needs no structured
// outputs support and therefore cannot fail the request.
func supportsOutputFormat(model string) bool {
	return matchesModel(model, StrictToolModels)
}

// PreservedThinkingModels retain all prior thinking by default, so accepting
// reasoning.context=all_turns requires no unsupported Converse parameter.
// Earlier Sonnet/Opus models and Haiku retain only the last turn by default.
// https://platform.claude.com/docs/en/build-with-claude/context-editing
var PreservedThinkingModels = []string{
	"fable-5",
	"mythos-5",
	"mythos-preview",

	"opus-4-5",
	"opus-4-6",
	"opus-4-7",
	"opus-4-8",
	"opus-5",

	"sonnet-4-6",
	"sonnet-5",
}

func matchesModel(model string, patterns []string) bool {
	model = strings.ToLower(model)

	for _, p := range patterns {
		if strings.Contains(model, p) {
			return true
		}
	}

	return false
}

func outputEffort(e provider.Effort) string {
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

// thinking is the resolved thinking configuration for one request.
type thinking struct {
	Enabled    bool
	Disabled   bool
	Summarized bool
	Updates    bool
	Effort     string
}

// resolveThinking maps the reasoning options onto the Converse additional
// fields. forced marks a request that forces a tool call (schema mode, tool
// choice "any"), which thinking is incompatible with over Bedrock. Thinking
// is also turned off when the last assistant turn holds tool calls without a
// signed thinking block, which Claude rejects.
func (c *Completer) resolveThinking(messages []provider.Message, options *provider.CompleteOptions, forced bool) thinking {
	if matchesModel(c.model, LegacyModels) {
		return thinking{}
	}

	var t thinking

	if r := options.ReasoningOptions; r != nil {
		t.Effort = outputEffort(r.Effort)
		t.Summarized = r.IncludeSummary
		t.Updates = r.IncludeUpdates && !r.IncludeSummary && matchesModel(c.model, ProgressUpdateModels)

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

	if matchesModel(c.model, AlwaysThinkingModels) {
		t.Disabled = false
	}

	// Display is only sent with an explicit adaptive config; without one, a
	// model that thinks by default uses its default display, "omitted".
	if !t.Enabled && !t.Disabled && !forced && (t.Summarized || t.Updates) &&
		(matchesModel(c.model, AlwaysThinkingModels) || matchesModel(c.model, DefaultThinkingModels)) {
		t.Enabled = true
	}

	if t.Disabled && matchesModel(c.model, DisabledThinkingEffortCapModels) && (t.Effort == "xhigh" || t.Effort == "max") {
		t.Effort = "high"
	}

	return t
}

// Check schema nodes, not annotations or property names: a property named
// "additionalProperties" is unrelated to the keyword on its containing schema.
func schemaAllowsAdditionalProperties(schema map[string]any) bool {
	if additional, ok := schema["additionalProperties"]; ok && additional != false {
		return true
	}
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties", "dependentSchemas"} {
		if children, ok := schema[key].(map[string]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok && schemaAllowsAdditionalProperties(nested) {
					return true
				}
			}
		}
	}
	for _, key := range []string{"items", "contains", "propertyNames", "not", "if", "then", "else", "additionalItems", "unevaluatedItems", "unevaluatedProperties"} {
		if nested, ok := schema[key].(map[string]any); ok && schemaAllowsAdditionalProperties(nested) {
			return true
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf", "prefixItems", "items"} {
		if children, ok := schema[key].([]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok && schemaAllowsAdditionalProperties(nested) {
					return true
				}
			}
		}
	}
	return false
}

func ensureAdditionalPropertiesFalse(schema map[string]any) map[string]any {
	if schema == nil {
		return schema
	}

	schemaType, _ := schema["type"].(string)
	if schemaType == "object" {
		if _, ok := schema["additionalProperties"]; !ok {
			schema["additionalProperties"] = false
		}

		if props, ok := schema["properties"].(map[string]any); ok {
			for key, val := range props {
				if propSchema, ok := val.(map[string]any); ok {
					props[key] = ensureAdditionalPropertiesFalse(propSchema)
				}
			}
		}
	}

	if schemaType == "array" {
		if items, ok := schema["items"].(map[string]any); ok {
			schema["items"] = ensureAdditionalPropertiesFalse(items)
		}
	}

	return schema
}

// Formats the strict-mode grammar compiler supports; others must be stripped.
var strictSupportedFormats = map[string]bool{
	"date-time": true, "time": true, "date": true, "duration": true,
	"email": true, "hostname": true, "uri": true,
	"ipv4": true, "ipv6": true, "uuid": true,
}

// Strict-mode schema validation rejects value-constraint keywords that other
// providers accept (numerical bounds, string lengths, array sizes, custom
// formats). Mirror the official SDKs: strip them client-side and fold them
// into the description so the model still sees the intent. Returns a copied
// schema along modified paths; the input is never mutated.
func sanitizeStrictSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	result := make(map[string]any, len(schema))

	var stripped []string

	for key, value := range schema {
		switch key {
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf",
			"minLength", "maxLength",
			"maxItems", "uniqueItems", "minContains", "maxContains",
			"minProperties", "maxProperties":
			stripped = append(stripped, fmt.Sprintf("%s %v", key, value))
			continue

		case "format":
			if s, ok := value.(string); ok && !strictSupportedFormats[s] {
				stripped = append(stripped, "format "+s)
				continue
			}
			result[key] = value

		case "minItems":
			// only 0 and 1 are supported
			if f, ok := value.(float64); ok && f <= 1 {
				result[key] = value
			} else {
				stripped = append(stripped, fmt.Sprintf("minItems %v", value))
			}

		// maps of named sub-schemas — keys are names, values are schemas
		case "properties", "$defs", "definitions", "patternProperties":
			if m, ok := value.(map[string]any); ok {
				sub := make(map[string]any, len(m))
				for name, propSchema := range m {
					if ps, ok := propSchema.(map[string]any); ok {
						sub[name] = sanitizeStrictSchema(ps)
					} else {
						sub[name] = propSchema
					}
				}
				result[key] = sub
			} else {
				result[key] = value
			}

		// single sub-schema values
		case "items", "contains", "propertyNames", "not", "if", "then", "else":
			if m, ok := value.(map[string]any); ok {
				result[key] = sanitizeStrictSchema(m)
			} else {
				result[key] = value
			}

		// lists of sub-schemas
		case "anyOf", "allOf", "oneOf", "prefixItems":
			if arr, ok := value.([]any); ok {
				sub := make([]any, len(arr))
				for i, item := range arr {
					if m, ok := item.(map[string]any); ok {
						sub[i] = sanitizeStrictSchema(m)
					} else {
						sub[i] = item
					}
				}
				result[key] = sub
			} else {
				result[key] = value
			}

		default:
			result[key] = value
		}
	}

	// keep the model aware of stripped constraints — they are no longer
	// grammar-enforced, but still guide generation
	if len(stripped) > 0 {
		hint := "Constraints: " + strings.Join(stripped, ", ")

		if desc, ok := result["description"].(string); ok && desc != "" {
			result["description"] = desc + " (" + hint + ")"
		} else {
			result["description"] = hint
		}
	}

	return simplifyNullableEnum(result)
}

// Anthropic rejects an enum next to a type list. When every enum value
// already satisfies the nullable type, that type is redundant: removing it
// preserves the enum and all sibling constraints without introducing anyOf.
func simplifyNullableEnum(schema map[string]any) map[string]any {
	types, ok := schema["type"].([]any)
	if !ok {
		return schema
	}

	values, ok := schema["enum"].([]any)
	if !ok {
		return schema
	}

	var others []any
	nullable := false

	for _, t := range types {
		if t == "null" {
			nullable = true
		} else {
			others = append(others, t)
		}
	}

	if !nullable || len(others) != 1 || len(values) == 0 {
		return schema
	}

	for _, v := range values {
		matches := false
		switch v := v.(type) {
		case nil:
			matches = true
		case string:
			matches = others[0] == "string"
		case bool:
			matches = others[0] == "boolean"
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			matches = others[0] == "integer" || others[0] == "number"
		case float64:
			matches = others[0] == "number" || others[0] == "integer" && math.Trunc(v) == v
		case float32:
			matches = others[0] == "number" || others[0] == "integer" && math.Trunc(float64(v)) == float64(v)
		}
		if !matches {
			return schema
		}
	}

	result := make(map[string]any, len(schema))

	for key, v := range schema {
		if key != "type" {
			result[key] = v
		}
	}

	return result
}
