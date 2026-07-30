package openai

import "github.com/adrianliechti/wingman/test/harness"

// DefaultTranscriptionResponseRules returns comparison rules for
// /v1/audio/transcriptions. Transcript text, timings and token counts vary per
// run, so only their presence is compared.
func DefaultTranscriptionResponseRules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"text": harness.FieldNonEmpty,

		"task": harness.FieldExact,

		"language": harness.FieldPresence,
		"duration": harness.FieldPresence,

		"languages": harness.FieldPresence,
		"logprobs":  harness.FieldIgnore,

		"segments": harness.FieldPresence,
		"words":    harness.FieldPresence,

		"usage": harness.FieldIgnore,
	}
}

// DefaultTranscriptionSSERules returns comparison rules for transcription SSE
// events.
func DefaultTranscriptionSSERules() map[string]harness.FieldRule {
	return map[string]harness.FieldRule{
		"type": harness.FieldExact,

		"delta": harness.FieldPresence,
		"text":  harness.FieldPresence,

		"id":      harness.FieldPresence,
		"speaker": harness.FieldPresence,
		"start":   harness.FieldPresence,
		"end":     harness.FieldPresence,

		"segment_id": harness.FieldIgnore,
		"languages":  harness.FieldPresence,
		"logprobs":   harness.FieldIgnore,

		"usage": harness.FieldIgnore,
	}
}
