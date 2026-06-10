package router

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, rejecting requests
	CircuitHalfOpen                     // Testing if recovered
)

// Default configuration values
const (
	DefaultFailureThreshold  = 5
	DefaultRecoveryTimeout   = 30 * time.Second
	DefaultFirstTokenTimeout = 2 * time.Minute

	latencyAlpha = 0.3 // EMA weight for TTFT
	errorAlpha   = 0.1 // EMA weight for error rate
)

// Metrics is a point-in-time snapshot of a provider's health
type Metrics struct {
	State     CircuitState
	TTFT      time.Duration
	ErrorRate float64
	Inflight  int64
}

// ProviderStats tracks health and performance metrics for a single provider
type ProviderStats struct {
	mu sync.Mutex

	avgTTFT time.Duration // EMA of time to first token
	hasTTFT bool
	// EMA of failure outcomes (0..1); decays so past incidents stop
	// influencing routing once a provider recovers
	errorRate float64

	inflight atomic.Int64

	state               CircuitState
	consecutiveFailures int
	lastFailure         time.Time
	// retryAfter extends the recovery wait beyond the configured timeout when
	// the last failure carried a Retry-After hint (e.g. Azure 429s)
	retryAfter time.Duration
	// probing marks an in-flight half-open probe. Recovery is gated on this
	// flag rather than the inflight count, so a hung request from before the
	// circuit opened cannot block recovery forever.
	probing bool
}

// NewProviderStats creates a new ProviderStats with default values
func NewProviderStats() *ProviderStats {
	return &ProviderStats{
		state:   CircuitClosed,
		avgTTFT: time.Second, // Initial estimate for time to first token
	}
}

// recovered reports whether an open circuit has waited out its recovery
// window. Must be called with the mutex held.
func (s *ProviderStats) recovered(recoveryTimeout time.Duration) bool {
	return time.Since(s.lastFailure) >= max(recoveryTimeout, s.retryAfter)
}

// IsCandidate reports whether the provider could serve a request right now.
// It is read-only and safe to call during scoring without claiming anything.
func (s *ProviderStats) IsCandidate(recoveryTimeout time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case CircuitOpen:
		return s.recovered(recoveryTimeout)

	case CircuitHalfOpen:
		return !s.probing

	default:
		return true
	}
}

// Acquire claims a request slot. For open circuits past the recovery timeout
// it transitions to half-open and claims the single probe slot; concurrent
// callers lose the race and must pick another provider. The returned probe
// flag marks ownership of that slot and must be passed back to exactly one
// RecordSuccess, RecordFailure or Release call - this keeps requests acquired
// before the circuit opened from interfering with a running probe.
func (s *ProviderStats) Acquire(recoveryTimeout time.Duration) (acquired, probe bool) {
	s.mu.Lock()

	switch s.state {
	case CircuitOpen:
		if !s.recovered(recoveryTimeout) {
			s.mu.Unlock()
			return false, false
		}

		s.state = CircuitHalfOpen
		s.probing = true
		probe = true

	case CircuitHalfOpen:
		if s.probing {
			s.mu.Unlock()
			return false, false
		}

		s.probing = true
		probe = true
	}

	s.mu.Unlock()

	s.inflight.Add(1)
	return true, probe
}

// Metrics returns a snapshot of the current health metrics
func (s *ProviderStats) Metrics() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()

	return Metrics{
		State:     s.state,
		TTFT:      s.avgTTFT,
		ErrorRate: s.errorRate,
		Inflight:  s.inflight.Load(),
	}
}

// RecordSuccess releases the slot and updates stats after a successful request
func (s *ProviderStats) RecordSuccess(ttft time.Duration, probe bool) {
	s.inflight.Add(-1)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.consecutiveFailures = 0

	if probe {
		s.probing = false
		s.state = CircuitClosed
	}

	s.errorRate *= 1 - errorAlpha

	if ttft > 0 {
		if !s.hasTTFT {
			s.avgTTFT = ttft
			s.hasTTFT = true
		} else {
			s.avgTTFT = time.Duration(float64(ttft)*latencyAlpha + float64(s.avgTTFT)*(1-latencyAlpha))
		}
	}
}

// RecordFailure releases the slot and updates stats after a failed request.
// A Retry-After hint on the error extends the recovery wait of an open
// circuit beyond the configured recovery timeout.
func (s *ProviderStats) RecordFailure(failureThreshold int, probe bool, err error) {
	s.inflight.Add(-1)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.consecutiveFailures++
	s.lastFailure = time.Now()
	s.retryAfter = provider.RetryAfterFromError(err)

	s.errorRate = s.errorRate*(1-errorAlpha) + errorAlpha

	if probe {
		// Failed probe: back to open, wait out another recovery window
		s.probing = false
		s.state = CircuitOpen
	} else if s.state == CircuitClosed && s.consecutiveFailures >= failureThreshold {
		s.state = CircuitOpen
	}
}

// Release frees the slot without counting the request as a success or failure.
// Used for outcomes that say nothing about provider health and must not open
// the circuit: the caller went away (context canceled) or the request itself
// was invalid (4xx). A released probe is inconclusive: the circuit stays
// half-open so the next request may probe again.
func (s *ProviderStats) Release(probe bool) {
	s.inflight.Add(-1)

	if !probe {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.probing = false
}
