package roundrobin

import (
	"context"
	"errors"
	"iter"
	"sync/atomic"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
)

// mockCompleter is a configurable mock for testing
type mockCompleter struct {
	err      error
	response string
	calls    atomic.Int64
}

func (m *mockCompleter) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		m.calls.Add(1)

		if m.err != nil {
			yield(nil, m.err)
			return
		}

		yield(&provider.Completion{
			ID: "test",
			Message: &provider.Message{
				Role: provider.MessageRoleAssistant,
				Content: []provider.Content{
					{Text: m.response},
				},
			},
		}, nil)
	}
}

func TestNewCompleter(t *testing.T) {
	t.Run("requires at least one completer", func(t *testing.T) {
		_, err := NewCompleter(nil)
		if err == nil {
			t.Error("expected error for empty completers")
		}
	})

	t.Run("routes to available provider", func(t *testing.T) {
		mock := &mockCompleter{response: "hello"}
		c, _ := NewCompleter([]provider.Completer{mock})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		var result *provider.Completion

		for completion, err := range c.Complete(ctx, messages, nil) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result = completion
		}

		if result == nil || result.Message.Text() != "hello" {
			t.Errorf("expected 'hello', got %v", result)
		}
	})
}

func TestDistribution(t *testing.T) {
	t.Run("rotates evenly among healthy providers", func(t *testing.T) {
		mocks := []*mockCompleter{
			{response: "one"},
			{response: "two"},
			{response: "three"},
		}

		c, _ := NewCompleter([]provider.Completer{mocks[0], mocks[1], mocks[2]})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		for range 300 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		for i, mock := range mocks {
			if calls := mock.calls.Load(); calls != 100 {
				t.Errorf("expected provider %d to receive exactly 100 of 300 calls, got %d", i, calls)
			}
		}
	})

	t.Run("skips open circuit providers", func(t *testing.T) {
		failing := &mockCompleter{err: errors.New("error")}
		healthy := &mockCompleter{response: "ok"}

		c, _ := NewCompleter([]provider.Completer{failing, healthy})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		for range 100 {
			for range c.Complete(ctx, messages, nil) {
			}

			if c.Stats()[0].Metrics().State == router.CircuitOpen {
				break
			}
		}

		if state := c.Stats()[0].Metrics().State; state != router.CircuitOpen {
			t.Fatalf("expected failing provider circuit to open, got %v", state)
		}

		failing.calls.Store(0)
		healthy.calls.Store(0)

		for range 10 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		if failing.calls.Load() > 0 {
			t.Error("failing provider should not receive calls while circuit is open")
		}

		if healthy.calls.Load() != 10 {
			t.Errorf("expected 10 calls to healthy provider, got %d", healthy.calls.Load())
		}
	})
}
