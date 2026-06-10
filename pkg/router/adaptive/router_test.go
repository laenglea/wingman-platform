package adaptive

import (
	"context"
	"errors"
	"iter"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// mockCompleter is a configurable mock for testing
type mockCompleter struct {
	delay    time.Duration
	err      error
	response string
	calls    atomic.Int64
}

func (m *mockCompleter) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		m.calls.Add(1)

		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			}
		}

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

func TestProviderSelection(t *testing.T) {
	t.Run("prefers lower TTFT provider", func(t *testing.T) {
		slow := &mockCompleter{response: "slow", delay: 100 * time.Millisecond}
		fast := &mockCompleter{response: "fast", delay: 10 * time.Millisecond}

		c, _ := NewCompleter([]provider.Completer{slow, fast})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Warm up until both providers have established TTFT metrics
		for range 200 {
			for range c.Complete(ctx, messages, nil) {
			}

			if slow.calls.Load() >= 2 && fast.calls.Load() >= 2 {
				break
			}
		}

		slow.calls.Store(0)
		fast.calls.Store(0)

		for range 100 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		if fast.calls.Load() <= slow.calls.Load() {
			t.Errorf("expected fast provider (%d calls) to be preferred over slow (%d calls)",
				fast.calls.Load(), slow.calls.Load())
		}
	})

	t.Run("shifts traffic away from failing provider", func(t *testing.T) {
		failing := &mockCompleter{err: errors.New("error")}
		healthy := &mockCompleter{response: "ok"}

		c, _ := NewCompleter([]provider.Completer{failing, healthy})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Warm up: the error rate and TTFT scoring should quickly starve the
		// failing provider, while failover keeps the requests successful
		for range 20 {
			for _, err := range c.Complete(ctx, messages, nil) {
				if err != nil {
					t.Fatalf("expected failover to absorb the failure, got: %v", err)
				}
			}
		}

		failing.calls.Store(0)
		healthy.calls.Store(0)

		for range 100 {
			for _, err := range c.Complete(ctx, messages, nil) {
				if err != nil {
					t.Fatalf("expected failover to absorb the failure, got: %v", err)
				}
			}
		}

		if healthy.calls.Load() != 100 {
			t.Errorf("expected healthy provider to serve all 100 requests, got %d", healthy.calls.Load())
		}

		if failing.calls.Load() > 10 {
			t.Errorf("expected failing provider to be mostly starved of traffic, got %d calls", failing.calls.Load())
		}
	})

	t.Run("inflight affects provider selection", func(t *testing.T) {
		mock1 := &mockCompleter{response: "one", delay: 5 * time.Millisecond}
		mock2 := &mockCompleter{response: "two", delay: 5 * time.Millisecond}

		c, _ := NewCompleter([]provider.Completer{mock1, mock2})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Warm up until both providers have established TTFT metrics
		for range 200 {
			for range c.Complete(ctx, messages, nil) {
			}

			if mock1.calls.Load() >= 2 && mock2.calls.Load() >= 2 {
				break
			}
		}

		// Simulate load on the first provider
		stats := c.Stats()
		stats[0].Acquire(0)
		stats[0].Acquire(0)
		stats[0].Acquire(0)

		mock1.calls.Store(0)
		mock2.calls.Store(0)

		for range 50 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		if mock2.calls.Load() <= mock1.calls.Load() {
			t.Errorf("expected less loaded provider to get more calls: got %d vs %d",
				mock2.calls.Load(), mock1.calls.Load())
		}
	})
}
