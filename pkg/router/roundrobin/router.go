package roundrobin

import (
	"context"
	"errors"
	"iter"
	"math/rand"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
)

// Completer implements a simple round-robin router with circuit breaker protection.
// Unlike the adaptive router, this does not track TTFT or inflight requests -
// it simply distributes load randomly among healthy providers.
type Completer struct {
	completers []provider.Completer
	stats      []*router.ProviderStats

	fallback provider.Completer

	failureThreshold int
	recoveryTimeout  time.Duration
}

type Option func(*Completer)

// WithFallback sets a fallback completer used when all primary providers are unavailable
func WithFallback(fallback provider.Completer) Option {
	return func(c *Completer) {
		c.fallback = fallback
	}
}

// NewCompleter creates a new round-robin router with circuit breaker protection
func NewCompleter(completers []provider.Completer, options ...Option) (provider.Completer, error) {
	if len(completers) == 0 {
		return nil, errors.New("at least one completer is required")
	}

	stats := make([]*router.ProviderStats, len(completers))
	for i := range stats {
		stats[i] = router.NewProviderStats()
	}

	c := &Completer{
		completers:       completers,
		stats:            stats,
		failureThreshold: router.DefaultFailureThreshold,
		recoveryTimeout:  router.DefaultRecoveryTimeout,
	}

	for _, option := range options {
		option(c)
	}

	return c, nil
}

// Complete routes the request to a randomly selected healthy provider
func (c *Completer) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		index := c.selectProvider()

		if index < 0 {
			if c.fallback != nil {
				for completion, err := range c.fallback.Complete(ctx, messages, options) {
					if !yield(completion, err) {
						return
					}
				}
				return
			}

			yield(nil, errors.New("all providers are unavailable"))
			return
		}

		var hasResponse bool

		for completion, err := range c.completers[index].Complete(ctx, messages, options) {
			if err != nil {
				if !yield(completion, err) {
					break
				}

				continue
			}

			hasResponse = true

			if !yield(completion, nil) {
				break
			}
		}

		// Update circuit breaker state only
		if hasResponse {
			c.stats[index].RecordSuccess(0, 0) // No TTFT tracking
		} else {
			c.stats[index].RecordFailure(c.failureThreshold)
		}
	}
}

// selectProvider randomly selects from available (healthy) providers
func (c *Completer) selectProvider() int {
	candidates := make([]int, 0, len(c.completers))

	for i, stat := range c.stats {
		if stat.IsAvailable(c.recoveryTimeout) {
			candidates = append(candidates, i)
		}
	}

	if len(candidates) == 0 {
		return -1
	}

	// Random selection among healthy providers
	return candidates[rand.Intn(len(candidates))]
}

