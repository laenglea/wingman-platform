package openai

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type responderLifecycleTransport func(*http.Request) (*http.Response, error)

func (f responderLifecycleTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type responderLifecycleBody struct {
	io.Reader
	closed             bool
	readsAfterTerminal int
}

func (b *responderLifecycleBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF {
		b.readsAfterTerminal++
	}
	return n, err
}
func (b *responderLifecycleBody) Close() error { b.closed = true; return nil }

func TestResponderStreamLifecycle(t *testing.T) {
	commentary := "data: " + `{"type":"response.output_text.delta","item_id":"msg_1","delta":"Checking."}` + "\n\n"
	for _, tc := range []struct {
		name, ending  string
		status        provider.CompletionStatus
		unexpectedEOF bool
		stop          []string
		errorMessage  string
		stopConsumer  bool
	}{
		{name: "EOF after commentary", unexpectedEOF: true},
		{name: "done without completion", ending: "data: [DONE]\n\n", unexpectedEOF: true},
		{name: "completed", ending: "data: " + `{"type":"response.completed","response":{"status":"completed","output":[]}}` + "\n\n", status: provider.CompletionStatusCompleted},
		{name: "incomplete", ending: "data: " + `{"type":"response.incomplete","response":{"status":"incomplete","output":[]}}` + "\n\n", status: provider.CompletionStatusIncomplete},
		{name: "requested stop", stop: []string{"Checking."}},
		{name: "consumer stops", stopConsumer: true},
		{name: "failed", ending: "data: " + `{"type":"response.failed","response":{"error":{"code":"server_error","message":"upstream failed"}}}` + "\n\n", errorMessage: "upstream failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &responderLifecycleBody{Reader: strings.NewReader(commentary + tc.ending)}
			client := &http.Client{Transport: responderLifecycleTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, Request: r}, nil
			})}
			responder, err := NewResponder("http://upstream.test", "test", WithToken("test"), WithClient(client))
			if err != nil {
				t.Fatal(err)
			}
			var runErr error
			var status provider.CompletionStatus
			for chunk, err := range responder.Complete(t.Context(), []provider.Message{provider.UserMessage("Go")}, &provider.CompleteOptions{Stop: tc.stop}) {
				if err != nil {
					runErr = err
					break
				}
				if chunk.Status != "" {
					status = chunk.Status
				}
				if tc.stopConsumer {
					break
				}
			}
			if tc.unexpectedEOF {
				if !errors.Is(runErr, io.ErrUnexpectedEOF) {
					t.Fatalf("error = %v, want unexpected EOF", runErr)
				}
			} else if tc.errorMessage != "" {
				if runErr == nil || !strings.Contains(runErr.Error(), tc.errorMessage) {
					t.Fatalf("error = %v, want %q", runErr, tc.errorMessage)
				}
			} else if runErr != nil {
				t.Fatal(runErr)
			}
			if tc.status != "" && status != tc.status {
				t.Fatalf("status = %q, want %q", status, tc.status)
			}
			if !body.closed {
				t.Error("upstream response body left open")
			}
			if tc.status != "" && body.readsAfterTerminal != 0 {
				t.Error("read past terminal event; an open HTTP body would stall completion")
			}
		})
	}
}
