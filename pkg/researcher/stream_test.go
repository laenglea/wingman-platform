package researcher_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/adrianliechti/wingman/pkg/researcher/anthropic"
	"github.com/adrianliechti/wingman/pkg/researcher/exa"
	"github.com/adrianliechti/wingman/pkg/researcher/openai"
	"github.com/adrianliechti/wingman/pkg/researcher/perplexity"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestStreamingProvidersReleaseResourcesWhenConsumerStops(t *testing.T) {
	for _, tt := range []struct {
		name   string
		event  string
		create func(*http.Client) (researcher.Provider, error)
	}{
		{"openai", `data: {"type":"response.in_progress"}`, func(client *http.Client) (researcher.Provider, error) {
			return openai.New("test", openai.WithClient(client))
		}},
		{"anthropic", "event: message_start\ndata: " + `{"type":"message_start","message":{"id":"test","content":[],"role":"assistant"}}`, func(client *http.Client) (researcher.Provider, error) {
			return anthropic.New("test", anthropic.WithClient(client))
		}},
		{"perplexity", `data: {"choices":[{"index":0,"delta":{"content":"Report"}}]}`, func(client *http.Client) (researcher.Provider, error) {
			return perplexity.New("test", perplexity.WithClient(client))
		}},
		{"exa", `data: {"type":"text-delta","delta":"Report"}`, func(client *http.Client) (researcher.Provider, error) { return exa.New("test", exa.WithClient(client)) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tt.event + "\n\n")}
			var requestContext context.Context
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestContext = req.Context()
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body}, nil
			})}
			p, err := tt.create(client)
			if err != nil {
				t.Fatal(err)
			}
			updates := 0
			for result, err := range p.Research(t.Context(), "q", nil) {
				if err != nil {
					t.Fatal(err)
				}
				if result.Progress == "" {
					t.Fatal("unexpected final report")
				}
				updates++
				if updates == 2 {
					break
				}
			}
			if updates != 2 || !body.closed || requestContext == nil || !errors.Is(requestContext.Err(), context.Canceled) {
				t.Fatalf("updates=%d closed=%v context=%v", updates, body.closed, requestContext)
			}
		})
	}
}
