package adaptive

import (
	"math/rand"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
)

// NewCompleter creates a smart router that scores providers by responsiveness
// (TTFT), error rate and current load, with circuit breaker protection and
// transparent failover
func NewCompleter(completers []provider.Completer, options ...router.Option) (*router.Completer, error) {
	return router.NewCompleter(completers, selectProvider, options...)
}

// explorationRate is the fraction of requests routed uniformly at random
// instead of by score. Without it a provider that warms up first dominates
// selection forever and cold or recovering providers never refresh their
// TTFT and error metrics.
const explorationRate = 0.1

// selectProvider performs weighted random selection: prefer lower TTFT, lower
// error rate and fewer inflight requests
func selectProvider(candidates []int, stats []*router.ProviderStats) int {
	if len(candidates) == 1 {
		return candidates[0]
	}

	if rand.Float64() < explorationRate {
		return candidates[rand.Intn(len(candidates))]
	}

	scores := make([]float64, len(candidates))

	var totalScore float64

	for n, i := range candidates {
		metrics := stats[i].Metrics()

		ttftMs := float64(metrics.TTFT.Milliseconds())

		if ttftMs < 1 {
			ttftMs = 1
		}

		// Inflight penalty: reduces score as concurrent requests increase.
		// This helps distribute load evenly and respect per-provider quotas
		inflightFactor := 1.0 / (1.0 + float64(metrics.Inflight))

		score := inflightFactor / (ttftMs * (1 + metrics.ErrorRate*10))

		// Penalize recovering circuits to limit probe traffic
		if metrics.State != router.CircuitClosed {
			score *= 0.1
		}

		scores[n] = score
		totalScore += score
	}

	if totalScore <= 0 {
		return candidates[rand.Intn(len(candidates))]
	}

	r := rand.Float64() * totalScore

	var cumulative float64

	for n, score := range scores {
		cumulative += score

		if r <= cumulative {
			return candidates[n]
		}
	}

	return candidates[len(candidates)-1]
}
