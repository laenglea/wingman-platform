package anthropic

import "github.com/adrianliechti/wingman/test/harness"

// DefaultMessagesResponseRules returns comparison rules for /v1/messages.
func DefaultMessagesResponseRules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"id":             harness.FieldPresence,
		"model":          harness.FieldIgnore,
		"content.*.text": harness.FieldIgnore,
		// Models choose different numbers of text and thinking blocks.
		// Feature tests assert the required text, calls, and signed state.
		"content": harness.FieldType,

		"usage.input_tokens":                          harness.FieldNonEmpty,
		"usage.output_tokens":                         harness.FieldNonEmpty,
		"usage.output_tokens_details":                 harness.FieldOptional,
		"usage.output_tokens_details.thinking_tokens": harness.FieldType,
		"usage.cache_creation":                        harness.FieldIgnore,
		"usage.cache_creation_input_tokens":           harness.FieldPresence,
		"usage.cache_read_input_tokens":               harness.FieldPresence,
		"usage.service_tier":                          harness.FieldIgnore,
		"usage.inference_geo":                         harness.FieldIgnore,
	}
}

// DefaultMessagesSSERules returns comparison rules for /v1/messages SSE events.
func DefaultMessagesSSERules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"message.id":                                harness.FieldPresence,
		"message.model":                             harness.FieldIgnore,
		"message.usage.input_tokens":                harness.FieldIgnore,
		"message.usage.output_tokens":               harness.FieldIgnore,
		"message.usage.cache_creation_input_tokens": harness.FieldIgnore,
		"message.usage.cache_read_input_tokens":     harness.FieldIgnore,
		"message.usage.cache_creation":              harness.FieldIgnore,
		"message.usage.service_tier":                harness.FieldIgnore,
		"message.usage.inference_geo":               harness.FieldIgnore,

		"delta.text":         harness.FieldIgnore,
		"delta.partial_json": harness.FieldIgnore,
		// Optional thinking blocks shift the text/tool indexes.
		"index": harness.FieldIgnore,

		"usage.input_tokens":                          harness.FieldIgnore,
		"usage.output_tokens":                         harness.FieldIgnore,
		"usage.output_tokens_details":                 harness.FieldOptional,
		"usage.output_tokens_details.thinking_tokens": harness.FieldType,
		"usage.cache_creation_input_tokens":           harness.FieldIgnore,
		"usage.cache_read_input_tokens":               harness.FieldIgnore,
	}
}
