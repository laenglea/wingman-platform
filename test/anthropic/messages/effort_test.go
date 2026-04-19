package messages_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/anthropic"
	"github.com/adrianliechti/wingman/test/harness"
)

func TestAdaptiveThinkingEffortHTTP(t *testing.T) {
	h := anthropic.New(t)

	efforts := []string{"low", "medium", "high", "max"}

	for _, effort := range efforts {
		t.Run(effort, func(t *testing.T) {
			for _, model := range anthropic.DefaultModels() {
				if !model.Capabilities.Thinking {
					continue
				}

				t.Run(model.Name, func(t *testing.T) {
					body := map[string]any{
						"max_tokens": 16000,
						"thinking": map[string]any{
							"type": "adaptive",
						},
						"output_config": map[string]any{
							"effort": effort,
						},
						"messages": []map[string]any{
							{"role": "user", "content": "How many r's are in strawberry?"},
						},
					}

					anthropicResp, wingmanResp := compareHTTP(t, h, model.Name, body)

					requireThinkingBlock(t, "anthropic", anthropicResp.Body)
					requireThinkingBlock(t, "wingman", wingmanResp.Body)

					rules := anthropic.DefaultMessagesResponseRules()
					rules["content.*.thinking"] = harness.FieldIgnore
					rules["content.*.signature"] = harness.FieldNonEmpty
					harness.CompareStructure(t, "response", anthropicResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
				})
			}
		})
	}
}

func TestAdaptiveThinkingEffortSSE(t *testing.T) {
	h := anthropic.New(t)

	efforts := []string{"low", "medium", "high", "max"}

	for _, effort := range efforts {
		t.Run(effort, func(t *testing.T) {
			for _, model := range anthropic.DefaultModels() {
				if !model.Capabilities.Thinking {
					continue
				}

				t.Run(model.Name, func(t *testing.T) {
					body := map[string]any{
						"max_tokens": 16000,
						"thinking": map[string]any{
							"type": "adaptive",
						},
						"output_config": map[string]any{
							"effort": effort,
						},
						"messages": []map[string]any{
							{"role": "user", "content": "How many r's are in strawberry?"},
						},
					}

					anthropicEvents, wingmanEvents := compareSSE(t, h, model.Name, body)

					requireThinkingSSEEvent(t, "anthropic", anthropicEvents)
					requireThinkingSSEEvent(t, "wingman", wingmanEvents)
				})
			}
		})
	}
}

func TestAdaptiveThinkingDisplayOmittedHTTP(t *testing.T) {
	h := anthropic.New(t)

	for _, model := range anthropic.DefaultModels() {
		if !model.Capabilities.Thinking {
			continue
		}

		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"max_tokens": 16000,
				"thinking": map[string]any{
					"type":    "adaptive",
					"display": "omitted",
				},
				"messages": []map[string]any{
					{"role": "user", "content": "How many r's are in strawberry?"},
				},
			}

			anthropicResp, wingmanResp := compareHTTP(t, h, model.Name, body)

			// With display=omitted, thinking blocks should still have a signature
			// but the thinking text should be empty
			requireThinkingBlockOmitted(t, "anthropic", anthropicResp.Body)
			requireThinkingBlockOmitted(t, "wingman", wingmanResp.Body)

			rules := anthropic.DefaultMessagesResponseRules()
			rules["content.*.thinking"] = harness.FieldIgnore
			rules["content.*.signature"] = harness.FieldNonEmpty
			harness.CompareStructure(t, "response", anthropicResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
		})
	}
}

func requireThinkingBlockOmitted(t *testing.T, label string, body map[string]any) {
	t.Helper()

	content, ok := body["content"].([]any)
	if !ok {
		t.Fatalf("[%s] content is not an array", label)
	}

	for _, block := range content {
		obj, ok := block.(map[string]any)
		if !ok {
			continue
		}

		if obj["type"] == "thinking" {
			thinking, _ := obj["thinking"].(string)
			if thinking != "" {
				t.Errorf("[%s] thinking block should be empty with display=omitted, got %q", label, thinking[:min(50, len(thinking))])
			}

			signature, _ := obj["signature"].(string)
			if signature == "" {
				t.Errorf("[%s] thinking block has empty signature", label)
			}

			return
		}
	}

	t.Fatalf("[%s] no thinking content block found", label)
}
