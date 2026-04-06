package roundrobin

import (
	"context"
	"errors"
	"iter"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
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
			time.Sleep(m.delay)
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

	t.Run("creates completer with providers", func(t *testing.T) {
		mock := &mockCompleter{response: "hello"}
		c, err := NewCompleter([]provider.Completer{mock})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Error("expected non-nil completer")
		}
	})
}

func TestComplete(t *testing.T) {
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

		if result == nil {
			t.Fatal("expected completion result")
		}

		if result.Message.Text() != "hello" {
			t.Errorf("expected 'hello', got '%s'", result.Message.Text())
		}
	})

	t.Run("records failure on error", func(t *testing.T) {
		mock := &mockCompleter{err: errors.New("provider error")}
		c, _ := NewCompleter([]provider.Completer{mock})
		comp := c.(*Completer)

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		for _, err := range c.Complete(ctx, messages, nil) {
			if err == nil {
				t.Error("expected error")
			}
		}

		state, _, _, failures, _ := comp.stats[0].GetMetrics()
		if failures != 1 {
			t.Errorf("expected 1 failure, got %d", failures)
		}
		if state != router.CircuitClosed {
			t.Errorf("expected circuit closed after 1 failure")
		}
	})

	t.Run("opens circuit after threshold failures", func(t *testing.T) {
		mock := &mockCompleter{err: errors.New("provider error")}
		c, _ := NewCompleter([]provider.Completer{mock})
		comp := c.(*Completer)

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Trigger failures to open circuit
		for range router.DefaultFailureThreshold {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		state, _, _, _, _ := comp.stats[0].GetMetrics()
		if state != router.CircuitOpen {
			t.Errorf("expected circuit open after %d failures", router.DefaultFailureThreshold)
		}
	})
}

func TestRandomDistribution(t *testing.T) {
	t.Run("distributes requests across providers", func(t *testing.T) {
		mock1 := &mockCompleter{response: "one"}
		mock2 := &mockCompleter{response: "two"}
		mock3 := &mockCompleter{response: "three"}

		c, _ := NewCompleter([]provider.Completer{mock1, mock2, mock3})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Run many requests
		for range 300 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		calls1 := mock1.calls.Load()
		calls2 := mock2.calls.Load()
		calls3 := mock3.calls.Load()

		// Each should get roughly 100 calls (with some variance)
		// Allow 50% variance for randomness
		for i, calls := range []int64{calls1, calls2, calls3} {
			if calls < 50 || calls > 150 {
				t.Errorf("provider %d got %d calls, expected roughly 100", i+1, calls)
			}
		}
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("skips open circuit providers", func(t *testing.T) {
		failing := &mockCompleter{err: errors.New("error")}
		healthy := &mockCompleter{response: "ok"}

		c, _ := NewCompleter([]provider.Completer{failing, healthy})

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Open circuit on first provider by triggering failures
		// We need to get the failing provider selected enough times
		for range 50 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		// Reset counts
		failing.calls.Store(0)
		healthy.calls.Store(0)

		// Wait a bit but less than recovery timeout
		time.Sleep(10 * time.Millisecond)

		// Next requests should only go to healthy provider
		for range 20 {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		// Healthy provider should get all or most calls
		healthyCalls := healthy.calls.Load()
		if healthyCalls < 15 {
			t.Errorf("expected most calls to healthy provider, got %d/20", healthyCalls)
		}
	})

	t.Run("recovers circuit after timeout", func(t *testing.T) {
		mock := &mockCompleter{err: errors.New("error")}
		c, _ := NewCompleter([]provider.Completer{mock})
		comp := c.(*Completer)

		// Use short recovery timeout for test
		comp.recoveryTimeout = 10 * time.Millisecond

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Open circuit
		for range router.DefaultFailureThreshold {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		state, _, _, _, _ := comp.stats[0].GetMetrics()
		if state != router.CircuitOpen {
			t.Fatal("expected circuit to be open")
		}

		// Wait for recovery timeout
		time.Sleep(20 * time.Millisecond)

		// Fix the provider
		mock.err = nil
		mock.response = "recovered"

		// Should transition to half-open and then closed on success
		var result *provider.Completion
		for completion, err := range c.Complete(ctx, messages, nil) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result = completion
		}

		if result.Message.Text() != "recovered" {
			t.Errorf("expected 'recovered', got '%s'", result.Message.Text())
		}

		state, _, _, _, _ = comp.stats[0].GetMetrics()
		if state != router.CircuitClosed {
			t.Errorf("expected circuit closed after recovery, got %v", state)
		}
	})
}

func TestFallback(t *testing.T) {
	t.Run("uses fallback when all providers unavailable", func(t *testing.T) {
		failing := &mockCompleter{err: errors.New("error")}
		fallback := &mockCompleter{response: "fallback"}

		c, _ := NewCompleter([]provider.Completer{failing}, WithFallback(fallback))
		comp := c.(*Completer)

		// Use long recovery timeout to prevent half-open
		comp.recoveryTimeout = time.Hour

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Open circuit
		for range router.DefaultFailureThreshold {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		// Next request should go to fallback
		var result *provider.Completion
		for completion, err := range c.Complete(ctx, messages, nil) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result = completion
		}

		if result.Message.Text() != "fallback" {
			t.Errorf("expected 'fallback', got '%s'", result.Message.Text())
		}
	})

	t.Run("returns error when no fallback and all unavailable", func(t *testing.T) {
		failing := &mockCompleter{err: errors.New("error")}

		c, _ := NewCompleter([]provider.Completer{failing})
		comp := c.(*Completer)

		comp.recoveryTimeout = time.Hour

		ctx := context.Background()
		messages := []provider.Message{provider.UserMessage("test")}

		// Open circuit
		for range router.DefaultFailureThreshold {
			for range c.Complete(ctx, messages, nil) {
			}
		}

		var gotError bool
		for _, err := range c.Complete(ctx, messages, nil) {
			if err != nil {
				gotError = true
			}
		}

		if !gotError {
			t.Error("expected error when all providers unavailable and no fallback")
		}
	})
}
