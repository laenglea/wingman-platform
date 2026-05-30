package responses_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/harness"
	"github.com/adrianliechti/wingman/test/openai"
)

// toolSearchClient is the client-executed flavor: the model asks for tools,
// the client returns them via tool_search_output on a follow-up turn.
var toolSearchClient = map[string]any{
	"type":        "tool_search",
	"execution":   "client",
	"description": "Find project-specific tools needed to satisfy the user's request.",
	"parameters": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal": map[string]any{
				"type":        "string",
				"description": "What the user is trying to accomplish.",
			},
		},
		"required": []string{"goal"},
	},
}

// deferredWeatherTool is a function tool hidden from the initial list. The
// model can only call it after surfacing it through tool_search.
var deferredWeatherTool = map[string]any{
	"type":          "function",
	"name":          "get_weather",
	"description":   "Get the current weather for a location.",
	"defer_loading": true,
	"parameters": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The city and country.",
			},
		},
		"required": []string{"location"},
	},
}

// TestToolSearchHTTP forwards a `tool_search` tool plus a `defer_loading: true`
// function and compares wingman's response shape to OpenAI's. The model may
// emit either a tool_search_call (when it decides to search) or a
// function_call (if it ignored the tool_search flow) — both endpoints must
// agree on the shape.
func TestToolSearchHTTP(t *testing.T) {
	h := openai.New(t)

	for _, model := range openai.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"input": "What's the weather in London? Use any tools you have available to find out.",
				"tools": []any{toolSearchClient, deferredWeatherTool},
			}

			openaiResp, wingmanResp := compareHTTP(t, h, model, body)

			rules := openai.DefaultResponsesResponseRules()
			rules["output"] = harness.FieldPresence
			rules["output.*.arguments"] = harness.FieldIgnore
			rules["tools"] = harness.FieldPresence
			harness.CompareStructure(t, "response", openaiResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
		})
	}
}

// TestToolSearchSSE is the streaming variant; the SSE event sequence and item
// shape must agree between wingman and OpenAI for the same request.
func TestToolSearchSSE(t *testing.T) {
	h := openai.New(t)

	for _, model := range openai.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"input": "What's the weather in London? Use any tools you have available to find out.",
				"tools": []any{toolSearchClient, deferredWeatherTool},
			}

			_, _ = compareSSE(t, h, model, body)
		})
	}
}

// TestToolSearchOutputRoundTrip exercises the multi-turn case: the prior turn
// is a tool_search_call/output pair, and the new turn asks the model to use
// the surfaced tool. Both endpoints must accept the round-trip input shape
// without error.
func TestToolSearchOutputRoundTripHTTP(t *testing.T) {
	h := openai.New(t)

	for _, model := range openai.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"tools": []any{toolSearchClient, deferredWeatherTool},
				"input": []map[string]any{
					{
						"type": "message",
						"role": "user",
						"content": []map[string]any{
							{"type": "input_text", "text": "What's the weather in London?"},
						},
					},
					{
						"type":      "tool_search_call",
						"id":        "tsc_prior",
						"call_id":   "call_prior_ts",
						"status":    "completed",
						"execution": "client",
						"arguments": map[string]any{"goal": "find a weather tool"},
					},
					{
						"type":      "tool_search_output",
						"call_id":   "call_prior_ts",
						"status":    "completed",
						"execution": "client",
						"tools":     []any{deferredWeatherTool},
					},
				},
			}

			openaiResp, wingmanResp := compareHTTP(t, h, model, body)

			rules := openai.DefaultResponsesResponseRules()
			rules["output"] = harness.FieldPresence
			rules["output.*.arguments"] = harness.FieldIgnore
			rules["tools"] = harness.FieldPresence
			harness.CompareStructure(t, "response", openaiResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
		})
	}
}
