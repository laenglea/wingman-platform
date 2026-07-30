package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func benchServer(b *testing.B) string {
	b.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "bench", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "greets",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Name string `json:"name"`
	}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "hi " + args.Name}},
		}, nil, nil
	})

	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true}))

	b.Cleanup(httpServer.Close)

	return httpServer.URL
}

// BenchmarkExecute_SessionPerCall measures the current wrapper, which opens a
// fresh MCP session for every tool call.
func BenchmarkExecute_SessionPerCall(b *testing.B) {
	url := benchServer(b)

	c, err := New(url, nil, nil)

	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()

	for b.Loop() {
		if _, err := c.Execute(ctx, "greet", map[string]any{"name": "x"}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExecute_ReusedSession measures the same call over a session opened
// once, as the SDK's own proxy example does.
func BenchmarkExecute_ReusedSession(b *testing.B) {
	url := benchServer(b)

	c, err := New(url, nil, nil)

	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	session, err := c.createSession(ctx)

	if err != nil {
		b.Fatal(err)
	}

	defer session.Close()

	b.ResetTimer()

	for b.Loop() {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"name": "x"},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTools_SessionPerCall(b *testing.B) {
	url := benchServer(b)

	c, err := New(url, nil, nil)

	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()

	for b.Loop() {
		if _, err := c.Tools(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
