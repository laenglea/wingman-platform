package responses_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/harness"
	"github.com/adrianliechti/wingman/test/openai"
)

// namespaceWeatherTool is a single function tool grouped under a Codex-style
// MCP namespace ("mcp__weather__"). Both OpenAI and wingman must accept the
// shape and surface tool calls with the namespace field intact.
var namespaceWeatherTool = map[string]any{
	"type":        "namespace",
	"name":        "mcp__weather__",
	"description": "Tools for working with the weather MCP server.",
	"tools": []any{
		map[string]any{
			"type":        "function",
			"name":        "get_weather",
			"description": "Get the current weather for a location.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "The city and country, e.g. 'London, UK'.",
					},
				},
				"required": []string{"location"},
			},
		},
	},
}

func TestNamespaceToolHTTP(t *testing.T) {
	h := openai.New(t)

	for _, model := range openai.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"input": "What's the weather in London?",
				"tools": []any{namespaceWeatherTool},
			}

			openaiResp, wingmanResp := compareHTTP(t, h, model, body)

			requireNamespacedFunctionCall(t, "openai", openaiResp.Body, "get_weather", "mcp__weather__")
			requireNamespacedFunctionCall(t, "wingman", wingmanResp.Body, "get_weather", "mcp__weather__")

			rules := openai.DefaultResponsesResponseRules()
			rules["output"] = harness.FieldPresence
			rules["tools"] = harness.FieldPresence
			harness.CompareStructure(t, "response", openaiResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
		})
	}
}

func TestNamespaceToolSSE(t *testing.T) {
	h := openai.New(t)

	for _, model := range openai.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"input": "What's the weather in London?",
				"tools": []any{namespaceWeatherTool},
			}

			openaiEvents, wingmanEvents := compareSSE(t, h, model, body)

			requireNamespacedFunctionCallSSE(t, "openai", openaiEvents, "get_weather", "mcp__weather__")
			requireNamespacedFunctionCallSSE(t, "wingman", wingmanEvents, "get_weather", "mcp__weather__")
		})
	}
}

func requireNamespacedFunctionCall(t *testing.T, label string, body map[string]any, name, namespace string) {
	t.Helper()

	output, ok := body["output"].([]any)
	if !ok {
		t.Fatalf("[%s] output is not an array: %+v", label, body["output"])
	}

	for _, item := range output {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if obj["type"] != "function_call" || obj["name"] != name {
			continue
		}
		if obj["namespace"] != namespace {
			t.Fatalf("[%s] function_call %q missing namespace %q: %+v", label, name, namespace, obj)
		}
		return
	}

	t.Fatalf("[%s] no namespaced function_call %q found in output: %+v", label, name, output)
}

func requireNamespacedFunctionCallSSE(t *testing.T, label string, events []*harness.SSEEvent, name, namespace string) {
	t.Helper()

	for _, ev := range events {
		if ev.Event != "response.output_item.done" {
			continue
		}
		item, ok := ev.Data["item"].(map[string]any)
		if !ok {
			continue
		}
		if item["type"] != "function_call" || item["name"] != name {
			continue
		}
		if item["namespace"] != namespace {
			t.Fatalf("[%s] streamed function_call %q missing namespace %q: %+v", label, name, namespace, item)
		}
		return
	}

	t.Fatalf("[%s] no streamed namespaced function_call %q found", label, name)
}
