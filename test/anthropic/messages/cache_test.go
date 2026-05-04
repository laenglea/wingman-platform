package messages_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/test/anthropic"
)

// TestPromptCacheRead verifies that wingman propagates cache_read_input_tokens
// on the second of two identical calls when cache_control is set.
//
// Caching is wingman-only: we don't compare against the reference Anthropic
// account because its independent cache state would diverge.
func TestPromptCacheRead(t *testing.T) {
	h := anthropic.New(t)

	for _, model := range anthropic.DefaultModels() {
		t.Run(model.Name, func(t *testing.T) {
			systemPrompt := buildLongSystemPrompt(t.Name())

			body := map[string]any{
				"max_tokens": 32,
				"system": []map[string]any{
					{
						"type":          "text",
						"text":          systemPrompt,
						"cache_control": map[string]any{"type": "ephemeral"},
					},
				},
				"messages": []map[string]any{
					{"role": "user", "content": "Reply with the single word: OK."},
				},
			}

			// First call — primes the cache.
			first := postAnthropic(t, h, h.Wingman, withModel(body, model.Name))
			if first.StatusCode != 200 {
				t.Fatalf("first call: status %d: %s", first.StatusCode, string(first.RawBody))
			}

			// Cache write propagation can take a moment; retry briefly.
			var lastUsage any
			for _, delay := range []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond, 3 * time.Second} {
				time.Sleep(delay)
				resp := postAnthropic(t, h, h.Wingman, withModel(body, model.Name))
				if resp.StatusCode != 200 {
					t.Fatalf("retry call: status %d: %s", resp.StatusCode, string(resp.RawBody))
				}
				lastUsage = resp.Body["usage"]
				if cacheRead := getInt(t, resp.Body, "usage", "cache_read_input_tokens"); cacheRead > 0 {
					return
				}
			}
			t.Fatalf("expected cache_read_input_tokens > 0 on a follow-up call, never observed\nfirst usage: %v\nlast usage: %v",
				first.Body["usage"], lastUsage)
		})
	}
}

// buildLongSystemPrompt produces a deterministic prompt long enough (>1024 tokens)
// to be eligible for Anthropic prompt caching. Including a stable per-test
// suffix isolates concurrent test runs from one another.
func buildLongSystemPrompt(seed string) string {
	var b strings.Builder
	b.WriteString("You are a careful assistant in test scenario ")
	b.WriteString(seed)
	b.WriteString(". Reference manual follows.\n\n")

	for i := range 200 {
		fmt.Fprintf(&b,
			"Section %d: This paragraph exists solely to inflate the token count "+
				"of the cached system prompt above the minimum threshold required "+
				"by the provider for prompt caching to take effect. The text is "+
				"intentionally repetitive and carries no operational meaning.\n",
			i)
	}
	return b.String()
}

func getInt(t *testing.T, m map[string]any, path ...string) int {
	t.Helper()
	var cur any = m
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return 0
		}
		cur = obj[key]
	}
	switch v := cur.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case nil:
		return 0
	default:
		return 0
	}
}
