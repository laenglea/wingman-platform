package bedrock

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// TestSanitizeStrictSchemaNullableEnum covers OpenAI's null-widened optional
// enums, which Anthropic's strict validator rejects as a type list + enum.
func TestSanitizeStrictSchemaNullableEnum(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"output_mode": map[string]any{
				"type":        []any{"string", "null"},
				"enum":        []any{"content", "files_with_matches", "count", nil},
				"description": "Output mode",
				"default":     "files_with_matches",
			},
			"plain": map[string]any{"type": "string", "enum": []any{"a", "b"}},
			"mixed": map[string]any{"type": []any{"string", "integer"}, "enum": []any{"a", float64(1)}},
		},
		"required":             []any{"output_mode", "plain", "mixed"},
		"additionalProperties": false,
	}

	props := sanitizeStrictSchema(schema)["properties"].(map[string]any)

	got, _ := json.Marshal(props["output_mode"])
	want := `{"default":"files_with_matches","description":"Output mode","enum":["content","files_with_matches","count",null]}`
	if string(got) != want {
		t.Errorf("nullable enum:\n got %s\nwant %s", got, want)
	}

	if got, _ := json.Marshal(props["plain"]); string(got) != `{"enum":["a","b"],"type":"string"}` {
		t.Errorf("plain enum changed: %s", got)
	}

	if got, _ := json.Marshal(props["mixed"]); string(got) != `{"enum":["a",1],"type":["string","integer"]}` {
		t.Errorf("non-nullable type list changed: %s", got)
	}

	if _, ok := schema["properties"].(map[string]any)["output_mode"].(map[string]any)["type"]; !ok {
		t.Error("input schema was mutated")
	}
}

// Validate values against both schemas so a rewrite cannot silently weaken
// enums or sibling constraints, and resolve references after the rewrite.
func TestSanitizeStrictSchemaNullableEnumConstraints(t *testing.T) {
	cases := []struct {
		name   string
		schema string
	}{
		{"nullable", `{"type":["string","null"],"enum":["a",null]}`},
		{"null excluded by enum", `{"type":["string","null"],"enum":["a","b"]}`},
		{"null only", `{"type":["string","null"],"enum":[null]}`},
		{"sibling const", `{"type":["string","null"],"enum":["a",null],"const":"a"}`},
		{"local definition", `{"type":["string","null"],"enum":["a",null],"$defs":{"choice":{"type":"string","enum":["a"]}},"$ref":"#/properties/mode/$defs/choice"}`},
		{"existing anyOf", `{"type":["string","null"],"enum":["a","b",null],"anyOf":[{"enum":["a",null]}]}`},
		{"existing anyOf and allOf", `{"type":["string","null"],"enum":["a","b",null],"anyOf":[{"enum":["a","b",null]}],"allOf":[{"enum":["b",null]}]}`},
		{"reference to existing anyOf", `{"type":["string","null"],"enum":["a",null],"anyOf":[{"enum":["a"]},{"type":"null"}],"$ref":"#/properties/mode/anyOf/0"}`},
		{"nullable integer", `{"type":["integer","null"],"enum":[1,null]}`},
		{"nullable number", `{"type":["number","null"],"enum":[1.5,null]}`},
		{"nullable boolean", `{"type":["boolean","null"],"enum":[true,null]}`},
		{"type restricts enum values", `{"type":["string","null"],"enum":["a",1,null]}`},
		{"integer excludes fractions", `{"type":["integer","null"],"enum":[1,1.5,null]}`},
	}

	resolve := func(t *testing.T, data []byte) *jsonschema.Resolved {
		t.Helper()
		var schema jsonschema.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			t.Fatalf("resolve %s: %v", data, err)
		}
		return resolved
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var field map[string]any
			if err := json.Unmarshal([]byte(tc.schema), &field); err != nil {
				t.Fatal(err)
			}
			schema := map[string]any{
				"type":                 "object",
				"properties":           map[string]any{"mode": field},
				"required":             []any{"mode"},
				"additionalProperties": false,
			}
			before, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(sanitizeStrictSchema(schema))
			if err != nil {
				t.Fatal(err)
			}
			original := resolve(t, before)
			sanitized := resolve(t, after)
			for _, value := range []any{nil, "a", "b", "c", float64(1), float64(2), float64(1.5), true} {
				instance := map[string]any{"mode": value}
				want := original.Validate(instance) == nil
				got := sanitized.Validate(instance) == nil
				if got != want {
					t.Errorf("value %#v: accepted before=%t after=%t; schema=%s", value, want, got, after)
				}
			}
			unchanged, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			if string(unchanged) != string(before) {
				t.Error("input schema was mutated")
			}
		})
	}
}
