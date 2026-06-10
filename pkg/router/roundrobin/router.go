package roundrobin

import (
	"sync/atomic"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
)

// NewCompleter creates a router that rotates requests evenly across healthy
// providers, with circuit breaker protection and transparent failover
func NewCompleter(completers []provider.Completer, options ...router.Option) (*router.Completer, error) {
	var next atomic.Uint64

	strategy := func(candidates []int, _ []*router.ProviderStats) int {
		return candidates[(next.Add(1)-1)%uint64(len(candidates))]
	}

	return router.NewCompleter(completers, strategy, options...)
}
