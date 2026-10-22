package compatibility_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/test/harness"
)

func TestResponsesCacheTTL(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	for _, stream := range []bool{false, true} {
		for _, breakpoint := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/breakpoint=%t", stream, breakpoint), func(t *testing.T) {
				for _, ep := range []harness.Endpoint{h.OpenAI, h.Wingman} {
					key := fmt.Sprintf("compat-%s-%d", ep.Name, time.Now().UnixNano())
					part := map[string]any{"type": "input_text", "text": key + "\n" + strings.Repeat("This reference describes a red square on a white canvas. ", 180)}
					cache := map[string]any{"ttl": "12h"}
					if breakpoint {
						cache = map[string]any{"mode": "explicit", "ttl": "30m"}
						part["prompt_cache_breakpoint"] = map[string]any{"mode": "explicit", "ttl": "12h"}
					}
					body := responseBody(model, []any{map[string]any{"role": "user", "content": []any{part, map[string]any{"type": "input_text", "text": "Reply only OK."}}}})
					body["prompt_cache_key"], body["prompt_cache_options"] = key, cache
					result := response(t, h.Client, ep, body, stream)
					details, _ := field(result, "usage", "input_tokens_details", "cache_write_tokens_details").(map[string]any)
					long, _ := details["ttl_12h_tokens"].(float64)
					short, _ := details["ttl_30m_tokens"].(float64)
					writes, _ := field(result, "usage", "input_tokens_details", "cache_write_tokens").(float64)
					if long <= 0 || long+short != writes {
						t.Errorf("%s: expected measured 12-hour writes and consistent total, got %v", ep.Name, field(result, "usage", "input_tokens_details"))
					}
				}
			})
		}
	}
}

func TestResponsesInvalidTTL(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	for _, breakpoint := range []bool{false, true} {
		t.Run(fmt.Sprintf("breakpoint=%t", breakpoint), func(t *testing.T) {
			body := responseBody(model, "Reply only OK.")
			body["prompt_cache_options"] = map[string]any{"ttl": "7d"}
			if breakpoint {
				body["prompt_cache_options"] = map[string]any{"mode": "explicit"}
				body["input"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "Reply only OK.", "prompt_cache_breakpoint": map[string]any{"mode": "explicit", "ttl": "7d"}}}}}
			}
			ref := post(t, h.Client, h.OpenAI, "/responses", body, nil)
			requireStatus(t, "reference", ref, 400)
			got := post(t, h.Client, h.Wingman, "/responses", body, nil)
			requireStatus(t, "wingman", got, ref.StatusCode)
		})
	}
}

func TestResponsesIncomplete(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	body := responseBody(model, "Write a 500-word story about an orchid. Do not shorten it.")
	body["max_output_tokens"] = 16
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			for _, ep := range []harness.Endpoint{h.OpenAI, h.Wingman} {
				result := response(t, h.Client, ep, body, stream)
				if result["status"] != "incomplete" || field(result, "incomplete_details", "reason") != "max_output_tokens" {
					t.Errorf("%s lost incomplete status/reason: %v", ep.Name, result)
				}
				input, _ := field(result, "usage", "input_tokens").(float64)
				output, _ := field(result, "usage", "output_tokens").(float64)
				if input <= 0 || output <= 0 || field(result, "usage", "total_tokens") != input+output {
					t.Errorf("%s incomplete usage: %v", ep.Name, result["usage"])
				}
			}
		})
	}
}

func TestResponsesStandard(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			body := responseBody(model, "Reply only OK.")
			body["service_tier"] = "default"
			reference := response(t, h.Client, h.OpenAI, body, stream)
			body["service_tier"] = "standard"
			actual := response(t, h.Client, h.Wingman, body, stream)
			if reference["service_tier"] != "default" || actual["service_tier"] != reference["service_tier"] {
				t.Errorf("effective service tier: reference=%v wingman=%v", reference["service_tier"], actual["service_tier"])
			}
		})
	}
}

func TestResponsesEffortMetadata(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	for _, explicit := range []bool{false, true} {
		body := responseBody(model, "Reply only OK.")
		if !explicit {
			delete(body, "reasoning")
		}
		reference := response(t, h.Client, h.OpenAI, body, false)
		actual := response(t, h.Client, h.Wingman, body, false)
		if explicit {
			if field(actual, "reasoning", "effort") != field(reference, "reasoning", "effort") {
				t.Fatalf("requested effort lost: reference=%v actual=%v", reference["reasoning"], actual["reasoning"])
			}
		} else if field(actual, "reasoning", "effort") != nil {
			t.Fatalf("invented a backend default effort: %v", actual["reasoning"])
		}
	}
}

func TestResponsesOutputReplay(t *testing.T) {
	const model = "gpt-6.1-sol"
	h := openAI(t, model, false)
	var referenceEncrypted bool
	for _, ep := range []harness.Endpoint{h.OpenAI, h.Wingman} {
		t.Run(ep.Name, func(t *testing.T) {
			input := []any{map[string]any{"role": "user", "content": "Remember the code ORCHID-731. Reply with only that code."}}
			body := responseBody(model, input)
			body["include"] = []string{"output.encrypted_content"}
			first := post(t, h.Client, ep, "/responses", body, nil)
			requireStatus(t, ep.Name, first, 200)
			output, _ := first.Body["output"].([]any)
			var encrypted bool
			for _, raw := range output {
				item := raw.(map[string]any)
				if item["type"] == "message" {
					value, _ := item["encrypted_content"].(string)
					encrypted = value != ""
				}
			}
			if ep.Name == h.OpenAI.Name {
				referenceEncrypted = encrypted
				t.Logf("reference emits encrypted message output: %t", encrypted)
			} else if encrypted != referenceEncrypted {
				t.Errorf("encrypted message output: reference=%t wingman=%t", referenceEncrypted, encrypted)
			}
			body["input"] = append(append(input, output...), map[string]any{"role": "user", "content": "What was the code? Reply only with the code."})
			second := post(t, h.Client, ep, "/responses", body, nil)
			requireStatus(t, ep.Name+" replay", second, 200)
			if !strings.Contains(string(second.RawBody), "ORCHID-731") {
				t.Errorf("%s replay lost the code", ep.Name)
			}
		})
	}
}

func responseBody(model string, input any) map[string]any {
	return map[string]any{"model": model, "input": input, "max_output_tokens": 256, "reasoning": map[string]any{"effort": "low"}, "store": false}
}
