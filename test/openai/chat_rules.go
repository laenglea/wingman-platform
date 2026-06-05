package openai

import "github.com/adrianliechti/wingman/test/harness"

// DefaultChatResponseRules returns comparison rules for /v1/chat/completions.
func DefaultChatResponseRules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"id":      harness.FieldPresence,
		"created": harness.FieldPresence,
		"model":   harness.FieldIgnore,

		"choices.*.message.content": harness.FieldIgnore,
		"choices.*.message.refusal": harness.FieldIgnore,

		"usage.prompt_tokens":     harness.FieldNonEmpty,
		"usage.completion_tokens": harness.FieldNonEmpty,
		"usage.total_tokens":      harness.FieldNonEmpty,

		"usage.prompt_tokens_details.cached_tokens": harness.FieldPresence,
		"usage.prompt_tokens_details.audio_tokens":  harness.FieldIgnore,

		"usage.completion_tokens_details.reasoning_tokens":           harness.FieldPresence,
		"usage.completion_tokens_details.audio_tokens":               harness.FieldIgnore,
		"usage.completion_tokens_details.accepted_prediction_tokens": harness.FieldIgnore,
		"usage.completion_tokens_details.rejected_prediction_tokens": harness.FieldIgnore,

		"service_tier":       harness.FieldPresence,
		"system_fingerprint": harness.FieldIgnore,
	}
}

// DefaultChatSSERules returns comparison rules for chat SSE events.
func DefaultChatSSERules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"id":      harness.FieldPresence,
		"created": harness.FieldPresence,
		"model":   harness.FieldIgnore,

		"choices.*.delta.content": harness.FieldIgnore,
		"choices.*.delta.refusal": harness.FieldIgnore,

		"service_tier":       harness.FieldPresence,
		"system_fingerprint": harness.FieldIgnore,
		"obfuscation":        harness.FieldIgnore,

		"usage.prompt_tokens":     harness.FieldNonEmpty,
		"usage.completion_tokens": harness.FieldNonEmpty,
		"usage.total_tokens":      harness.FieldNonEmpty,

		"usage.prompt_tokens_details.cached_tokens": harness.FieldPresence,
		"usage.prompt_tokens_details.audio_tokens":  harness.FieldIgnore,

		"usage.completion_tokens_details.reasoning_tokens":           harness.FieldPresence,
		"usage.completion_tokens_details.audio_tokens":               harness.FieldIgnore,
		"usage.completion_tokens_details.accepted_prediction_tokens": harness.FieldIgnore,
		"usage.completion_tokens_details.rejected_prediction_tokens": harness.FieldIgnore,
	}
}
