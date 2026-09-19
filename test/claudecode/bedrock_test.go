package claudecode

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"slices"
	"testing"

	"github.com/adrianliechti/wingman/config"
	"github.com/adrianliechti/wingman/pkg/policy/noop"
	"github.com/adrianliechti/wingman/pkg/provider/bedrock"
	server "github.com/adrianliechti/wingman/server/anthropic"
	"github.com/adrianliechti/wingman/test/harness"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/go-chi/chi/v5"
)

type bedrockTransport func(*http.Request) (*http.Response, error)

func (f bedrockTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Run the real CLI, Wingman handler and Bedrock adapter. Newer Claude Code
// sends a trailing system message and keep-all thinking retention; Converse
// must receive those instructions at the top level and end on the user turn.
func TestClaudeCodeOfflineBedrock(t *testing.T) {
	binary, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("Claude Code is not installed")
	}
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "")

	var wire bytes.Buffer
	encoder := eventstream.NewEncoder()
	for _, event := range []struct{ kind, payload string }{
		{"messageStart", `{"role":"assistant"}`},
		{"contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"WINGMAN_E2E_OK"}}`},
		{"contentBlockStop", `{"contentBlockIndex":0}`},
		{"messageStop", `{"stopReason":"end_turn"}`},
		{"metadata", `{"usage":{"inputTokens":10,"outputTokens":5,"totalTokens":15},"metrics":{"latencyMs":1}}`},
	} {
		if err := encoder.Encode(&wire, eventstream.Message{
			Headers: eventstream.Headers{
				{Name: ":message-type", Value: eventstream.StringValue("event")},
				{Name: ":event-type", Value: eventstream.StringValue(event.kind)},
				{Name: ":content-type", Value: eventstream.StringValue("application/json")},
			},
			Payload: []byte(event.payload),
		}); err != nil {
			t.Fatal(err)
		}
	}

	var systemTexts []string
	client := &http.Client{Transport: bedrockTransport(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		var request struct {
			System   []struct{ Text string }
			Messages []struct{ Role string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, err
		}
		if len(request.Messages) == 0 || request.Messages[len(request.Messages)-1].Role != "user" {
			t.Error("Bedrock conversation must end with a user message")
		}
		for _, message := range request.Messages {
			if message.Role == "system" {
				t.Error("Converse does not support mid-conversation system messages")
			}
		}
		for _, block := range request.System {
			systemTexts = append(systemTexts, block.Text)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}}, Body: io.NopCloser(bytes.NewReader(wire.Bytes())), Request: r}, nil
	})}
	p, err := bedrock.NewCompleter("eu.anthropic.claude-opus-5", bedrock.WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	const model = "claude-opus-5"
	cfg := &config.Config{Policy: noop.New()}
	cfg.RegisterCompleter(model, p)
	router := chi.NewRouter()
	router.Route("/v1", server.New(cfg).Attach)
	upstream := httptest.NewServer(router)
	t.Cleanup(upstream.Close)
	answer, exchanges, _ := runClaude(t, binary, harness.Endpoint{Name: "offline", BaseURL: upstream.URL + "/v1", APIKey: "test"}, model, "Reply with WINGMAN_E2E_OK.", "", "")
	if answer != "WINGMAN_E2E_OK" {
		t.Errorf("unexpected CLI output: %q", answer)
	}
	checkExchanges(t, exchanges, model)

	sawSystem := false
	for _, exchange := range exchanges {
		u, err := url.Parse(exchange.Path)
		if err != nil || u.Path != "/v1/messages" {
			continue
		}
		var request struct {
			Messages []struct {
				Role    string
				Content json.RawMessage
			}
		}
		if err := json.Unmarshal([]byte(exchange.Request), &request); err != nil {
			t.Fatal(err)
		}
		for _, message := range request.Messages {
			if message.Role != "system" {
				continue
			}
			sawSystem = true
			var blocks []struct{ Text string }
			if err := json.Unmarshal(message.Content, &blocks); err != nil {
				t.Fatal(err)
			}
			for _, block := range blocks {
				if !slices.Contains(systemTexts, block.Text) {
					t.Errorf("lost CLI system instruction %q", block.Text)
				}
			}
		}
	}
	if !sawSystem {
		t.Fatal("installed CLI did not exercise mid-conversation system messages")
	}
}
