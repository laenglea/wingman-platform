package errors

import (
	"context"
	"iter"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/test/harness"
)

type refusalStatusCompleter struct{ explicit bool }

func (c refusalStatusCompleter) Complete(_ context.Context, _ []provider.Message, _ *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		content := provider.TextContent("Cannot help.")
		if c.explicit {
			content = provider.RefusalContent("Cannot help.")
		}
		if !yield(&provider.Completion{Message: &provider.Message{Role: provider.MessageRoleAssistant, Content: []provider.Content{content}}}, nil) {
			return
		}
		yield(&provider.Completion{Status: provider.CompletionStatusRefused}, nil)
	}
}

func TestChatRefusalAndContentFilterFinishReasons(t *testing.T) {
	client := harness.NewClient()
	for _, explicit := range []bool{false, true} {
		srv := newChatServer(refusalStatusCompleter{explicit: explicit}, "test-model")
		defer srv.Close()
		want := "content_filter"
		if explicit {
			want = "stop"
		}
		for _, stream := range []bool{false, true} {
			body := map[string]any{"model": "test-model", "stream": stream, "messages": []map[string]any{{"role": "user", "content": "x"}}}
			ep := harness.Endpoint{Name: "wingman", BaseURL: srv.URL + "/v1"}
			var reason string
			if stream {
				events, err := client.PostSSE(t.Context(), ep, "/chat/completions", body)
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range events {
					choices, _ := e.Data["choices"].([]any)
					if len(choices) > 0 {
						if value, ok := choices[0].(map[string]any)["finish_reason"].(string); ok && value != "" {
							reason = value
						}
					}
				}
			} else {
				resp, err := client.Post(t.Context(), ep, "/chat/completions", body)
				if err != nil {
					t.Fatal(err)
				}
				reason = resp.Body["choices"].([]any)[0].(map[string]any)["finish_reason"].(string)
			}
			if reason != want {
				t.Errorf("explicit=%t stream=%t: finish_reason=%q, want %q", explicit, stream, reason, want)
			}
		}
	}
}
