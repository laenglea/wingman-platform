package otel

import (
	"context"
	"iter"

	"github.com/adrianliechti/wingman/pkg/researcher"

	"go.opentelemetry.io/otel"
)

type Researcher interface {
	Observable
	researcher.Provider
}

type observableResearcher struct {
	model    string
	provider string

	researcher researcher.Provider
}

func NewResearcher(provider, model string, p researcher.Provider) Researcher {
	return &observableResearcher{
		researcher: p,

		model:    model,
		provider: provider,
	}
}

func (p *observableResearcher) otelSetup() {
}

func (p *observableResearcher) Research(ctx context.Context, instructions string, options *researcher.ResearchOptions) iter.Seq2[*researcher.Result, error] {
	return func(yield func(*researcher.Result, error) bool) {
		ctx, span := otel.Tracer(instrumentationName).Start(ctx, "research "+p.model)
		defer span.End()
		for result, err := range p.researcher.Research(ctx, instructions, options) {
			if err != nil {
				RecordError(span, err)
				yield(nil, err)
				return
			}
			if !yield(result, nil) {
				return
			}
		}
	}
}
