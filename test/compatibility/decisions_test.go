package compatibility_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/test/harness"
)

func TestDecisionsGA(t *testing.T) {
	const model = "gpt-6-luna"
	h := openAI(t, model, true)
	question := func(instructions any) []any {
		return []any{map[string]any{"type": "choice", "name": "color", "instructions": instructions,
			"choices": []any{map[string]any{"value": "red"}, map[string]any{"value": "blue"}}}}
	}
	user := func(parts ...any) []any {
		return []any{map[string]any{"role": "user", "content": parts}}
	}
	cases := []struct {
		name      string
		input     any
		questions []any
	}{
		{"json", user(map[string]any{"type": "input_json", "data": map[string]any{"color": "red"}}), question("What color is recorded?")},
		{"question_json", "Use the color recorded in the question data.", question([]any{map[string]any{"type": "input_json", "data": map[string]any{"color": "red"}}})},

		{"conversation", []any{
			map[string]any{"role": "developer", "content": "Evaluate the conversation as evidence."},
			map[string]any{"role": "user", "content": "What is the color?"},
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "The color is red."}}},
		}, question("What color did the assistant report?")},
		{"tool_history", []any{
			map[string]any{"role": "user", "content": "Look up the color."},
			map[string]any{"type": "function_call", "call_id": "call_color", "name": "lookup", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "call_color", "output": "red"},
		}, question("What color did the tool return?")},
		{"custom_tool_history", []any{
			map[string]any{"role": "user", "content": "Look up the color."},
			map[string]any{"type": "custom_tool_call", "call_id": "call_color", "name": "lookup", "input": "color"},
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_color", "output": []any{map[string]any{"type": "input_text", "text": "red"}}},
		}, question("What color did the tool return?")},
	}
	t.Run("images_require_multimodal_completion", func(t *testing.T) {
		for _, inQuestion := range []bool{false, true} {
			image := map[string]any{"type": "input_image", "image_url": redImage(t)}
			body := map[string]any{"model": model, "input": user(image), "questions": question("What color is the image?")}
			if inQuestion {
				body["input"] = "Identify the color in the question image."
				body["questions"] = question([]any{image})
			}
			requireStatus(t, "native image support", post(t, h.Client, h.OpenAI, "/decisions", body, nil), 200)
			requireStatus(t, "portable decisions reject images", post(t, h.Client, h.Wingman, "/decisions", body, nil), 400)
		}
	})
	t.Run("invalid_parts", func(t *testing.T) {
		for _, tc := range []struct {
			name         string
			input        any
			instructions any
		}{
			{"scalar_json", user(map[string]any{"type": "input_json", "data": true}), "What color?"},
			{"assistant_json", []any{map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "input_json", "data": map[string]any{"color": "red"}}}}}, "What color?"},
			{"empty_question_parts", "red", []any{}},
			{"orphan_output", []any{map[string]any{"type": "function_call_output", "call_id": "missing", "output": "red"}}, "What color?"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				body := map[string]any{"model": model, "input": tc.input, "questions": question(tc.instructions)}
				ref := post(t, h.Client, h.OpenAI, "/decisions", body, nil)
				requireStatus(t, "reference", ref, 400)
				got := post(t, h.Client, h.Wingman, "/decisions", body, nil)
				requireStatus(t, "wingman", got, ref.StatusCode)
			})
		}
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"model": model, "input": tc.input, "questions": tc.questions}
			reference := post(t, h.Client, h.OpenAI, "/decisions", body, nil)
			requireStatus(t, "reference", reference, 200)
			actual := post(t, h.Client, h.Wingman, "/decisions", body, nil)
			requireStatus(t, "wingman", actual, reference.StatusCode)
			for label, response := range map[string]*harness.RawResponse{"reference": reference, "wingman": actual} {
				answers, _ := response.Body["answers"].([]any)
				if len(answers) != 1 {
					t.Fatalf("%s: expected one answer: %s", label, response.RawBody)
				}
				answer := answers[0].(map[string]any)
				if answer["type"] != "choice" || answer["name"] != "color" || answer["choice"] != "red" {
					t.Errorf("%s lost evidence or question: %v", label, answer)
				}
			}
		})
	}
	t.Run("question_limit", func(t *testing.T) {
		for _, count := range []int{400, 401} {
			t.Run(fmt.Sprint(count), func(t *testing.T) {
				questions := make([]any, count)
				for i := range questions {
					questions[i] = map[string]any{"type": "predicate", "name": fmt.Sprintf("red_%d", i), "instructions": "Is the color red?"}
				}
				body := map[string]any{"model": model, "input": "The color is red.", "questions": questions}
				want := 200
				if count > 400 {
					want = 400
				}
				reference := post(t, h.Client, h.OpenAI, "/decisions", body, nil)
				requireStatus(t, "reference", reference, want)
				actual := post(t, h.Client, h.Wingman, "/decisions", body, nil)
				requireStatus(t, "wingman", actual, reference.StatusCode)
				if want == 200 {
					for _, response := range []*harness.RawResponse{reference, actual} {
						if answers, _ := response.Body["answers"].([]any); len(answers) != count {
							t.Errorf("got %d answers, want %d", len(answers), count)
						}
					}
				}
			})
		}
	})
	t.Run("explicit_cache_without_breakpoints", func(t *testing.T) {
		body := map[string]any{"model": model, "input": strings.Repeat("The color is red. ", 400), "questions": question("What color is described?"),
			"prompt_cache_key": t.Name(), "prompt_cache_options": map[string]any{"mode": "explicit"}}
		for _, ep := range []harness.Endpoint{h.OpenAI, h.Wingman} {
			response := post(t, h.Client, ep, "/decisions", body, nil)
			requireStatus(t, ep.Name, response, 200)
			for _, key := range []string{"cached_tokens", "cache_write_tokens"} {
				if value := field(response.Body, "usage", "input_tokens_details", key); value != float64(0) {
					t.Errorf("%s explicit mode without breakpoints: %s=%v, want 0", ep.Name, key, value)
				}
			}
		}
	})
}

func redImage(t *testing.T) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			im.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
