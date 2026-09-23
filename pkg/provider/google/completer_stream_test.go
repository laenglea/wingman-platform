package google

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func completeAll(t *testing.T, handler http.HandlerFunc, options *provider.CompleteOptions) ([]*provider.Completion, error) {
	t.Helper()

	c, err := NewCompleter("gemini-3.8-flash", WithToken("test-token"), WithClient(newTestClient(t, handler)))

	if err != nil {
		t.Fatal(err)
	}

	var deltas []*provider.Completion

	for delta, err := range c.Complete(context.Background(), []provider.Message{provider.UserMessage("hi")}, options) {
		if err != nil {
			return deltas, err
		}

		deltas = append(deltas, delta)
	}

	return deltas, nil
}

func sseEvent(data string) string {
	return "data: " + data + "\n\n"
}

func TestComplete_PromptBlocked(t *testing.T) {
	deltas, err := completeAll(t, serveSSE(sseEvent(`{"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"},"responseId":"r1"}`)), &provider.CompleteOptions{})

	if err != nil {
		t.Fatal(err)
	}

	if len(deltas) != 1 || deltas[0].Status != provider.CompletionStatusRefused || deltas[0].StopDetails.Category != "PROHIBITED_CONTENT" {
		t.Fatalf("expected a refusal, got %+v", deltas)
	}
}

func TestComplete_CandidateWithoutContent(t *testing.T) {
	deltas, err := completeAll(t, serveSSE(sseEvent(`{"candidates":[{"finishReason":"SAFETY","index":0}],"responseId":"r1"}`)), &provider.CompleteOptions{})

	if err != nil {
		t.Fatal(err)
	}

	if len(deltas) != 1 || deltas[0].StopReason != provider.StopReasonRefusal {
		t.Fatalf("expected a refusal, got %+v", deltas)
	}
}

func TestComplete_RetriesUnsupportedMinimalThinking(t *testing.T) {
	var levels []string

	handler := func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GenerationConfig struct {
				ThinkingConfig struct {
					ThinkingLevel string `json:"thinkingLevel"`
				} `json:"thinkingConfig"`
			} `json:"generationConfig"`
		}

		json.NewDecoder(r.Body).Decode(&body)
		levels = append(levels, body.GenerationConfig.ThinkingConfig.ThinkingLevel)

		if len(levels) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":{"code":400,"message":"Thinking level MINIMAL is not supported for this model. Please retry with other thinking level.","status":"INVALID_ARGUMENT"}}`)
			return
		}

		serveSSE(sseEvent(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`))(w, r)
	}

	options := &provider.CompleteOptions{
		ReasoningOptions: &provider.ReasoningOptions{Type: provider.ReasoningTypeDisabled},
	}

	deltas, err := completeAll(t, handler, options)

	if err != nil {
		t.Fatal(err)
	}

	if len(levels) != 2 || levels[0] != "MINIMAL" || levels[1] != "LOW" {
		t.Fatalf("expected MINIMAL then LOW, got %v", levels)
	}

	if len(deltas) != 1 || deltas[0].StopReason != provider.StopReasonEndTurn {
		t.Fatalf("unexpected deltas %+v", deltas)
	}
}
