package schema

// CloseObjects adds additionalProperties: false where it is omitted from object
// schemas, including nested properties and array items. It modifies the schema
// in place and preserves explicit additionalProperties values.
func CloseObjects(schema map[string]any) map[string]any {
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
					props[key] = CloseObjects(propSchema)
				}
			}
		}
	}

	if schemaType == "array" {
		if items, ok := schema["items"].(map[string]any); ok {
			schema["items"] = CloseObjects(items)
		}
	}

	return schema
}

// AllowsAdditionalProperties checks schema nodes for open objects, ignoring
// annotations and property names that happen to match schema keywords.
func AllowsAdditionalProperties(schema map[string]any) bool {
	if additional, ok := schema["additionalProperties"]; ok && additional != false {
		return true
	}
	for _, key := range []string{"properties", "$defs", "definitions", "patternProperties", "dependentSchemas"} {
		if children, ok := schema[key].(map[string]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok && AllowsAdditionalProperties(nested) {
					return true
				}
			}
		}
	}
	for _, key := range []string{"items", "contains", "propertyNames", "not", "if", "then", "else", "additionalItems", "unevaluatedItems", "unevaluatedProperties"} {
		if nested, ok := schema[key].(map[string]any); ok && AllowsAdditionalProperties(nested) {
			return true
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf", "prefixItems", "items"} {
		if children, ok := schema[key].([]any); ok {
			for _, child := range children {
				if nested, ok := child.(map[string]any); ok && AllowsAdditionalProperties(nested) {
					return true
				}
			}
		}
	}
	return false
}
