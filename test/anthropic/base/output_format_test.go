package base_test

import (
	"encoding/json"
	"testing"

	"github.com/adrianliechti/wingman/test/anthropic"
	"github.com/adrianliechti/wingman/test/harness"
)

var recipeSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"name":     map[string]any{"type": "string"},
		"servings": map[string]any{"type": "integer"},
	},
	"required":             []string{"name", "servings"},
	"additionalProperties": false,
}

// TestOutputConfigFormatHTTP exercises structured output via the canonical
// output_config.format parameter (what current SDKs send) instead of the
// deprecated top-level output_format.
func TestOutputConfigFormatHTTP(t *testing.T) {
	h := anthropic.New(t)

	for _, model := range anthropic.DefaultModels() {
		if !model.Capabilities.StructuredOutput {
			continue
		}

		t.Run(model.Name, func(t *testing.T) {
			body := map[string]any{
				"max_tokens": 1024,
				"messages": []map[string]any{
					{"role": "user", "content": "Suggest a simple pasta recipe."},
				},
				"output_config": map[string]any{
					"format": map[string]any{
						"type":   "json_schema",
						"schema": recipeSchema,
					},
				},
			}

			anthropicResp, wingmanResp := anthropic.CompareHTTP(t, h, model.Name, body)

			requireValidRecipeText(t, "anthropic", anthropicResp.Body)
			requireValidRecipeText(t, "wingman", wingmanResp.Body)

			rules := anthropic.DefaultMessagesResponseRules()
			harness.CompareStructure(t, "response", anthropicResp.Body, wingmanResp.Body, harness.CompareOption{Rules: rules})
		})
	}
}

func requireValidRecipeText(t *testing.T, label string, body map[string]any) {
	t.Helper()

	content, ok := body["content"].([]any)
	if !ok {
		t.Fatalf("[%s] content is not an array", label)
	}

	for _, block := range content {
		obj, _ := block.(map[string]any)
		if obj["type"] != "text" {
			continue
		}

		text, _ := obj["text"].(string)
		if text == "" {
			continue
		}

		var recipe struct {
			Name     string `json:"name"`
			Servings int    `json:"servings"`
		}

		if err := json.Unmarshal([]byte(text), &recipe); err != nil {
			t.Fatalf("[%s] text is not valid JSON: %v\ntext: %s", label, err, text)
		}

		if recipe.Name == "" {
			t.Errorf("[%s] recipe name is empty", label)
		}
		if recipe.Servings <= 0 {
			t.Errorf("[%s] recipe servings is %d", label, recipe.Servings)
		}

		return
	}

	t.Fatalf("[%s] no text content block found", label)
}
