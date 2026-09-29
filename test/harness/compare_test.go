package harness

import "testing"

func TestOptionalFieldsStillCheckTypesAndRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name             string
		expected, actual map[string]any
		wantDiff         bool
	}{
		{"missing optional", map[string]any{"details": map[string]any{}}, map[string]any{}, false},
		{"extra optional", map[string]any{}, map[string]any{"details": map[string]any{}}, false},
		{"same type", map[string]any{"details": map[string]any{"tokens": float64(1)}}, map[string]any{"details": map[string]any{"tokens": float64(9)}}, false},
		{"wrong type", map[string]any{"details": map[string]any{}}, map[string]any{"details": "invalid"}, true},
		{"missing nested field", map[string]any{"details": map[string]any{"tokens": float64(1)}}, map[string]any{"details": map[string]any{}}, true},
		{"invalid nested field", map[string]any{"details": map[string]any{"tokens": float64(1)}}, map[string]any{"details": map[string]any{"tokens": "invalid"}}, true},
		{"missing required", map[string]any{"status": "completed"}, map[string]any{}, true},
		{"changed required", map[string]any{"status": "completed"}, map[string]any{"status": "failed"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diffs []string
			compareMap(t, &diffs, "", tc.expected, tc.actual, CompareOption{Rules: map[string]FieldRule{"details": FieldOptional, "details.tokens": FieldType}})
			if (len(diffs) > 0) != tc.wantDiff {
				t.Fatalf("diffs = %v", diffs)
			}
		})
	}
}
