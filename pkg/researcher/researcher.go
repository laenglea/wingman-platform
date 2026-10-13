package researcher

import (
	"context"
	"errors"
	"iter"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type Provider interface {
	Research(ctx context.Context, instructions string, options *ResearchOptions) iter.Seq2[*Result, error]
}

type Effort = provider.Effort
type Verbosity = provider.Verbosity

type ResearchOptions struct {
}

type Result struct {
	// Progress describes an intermediate step. Results with Progress set are
	// updates; the final result has empty Progress and contains the full report.
	Progress string
	Content  string
}

// Collect consumes progress updates and returns the final report.
func Collect(results iter.Seq2[*Result, error]) (*Result, error) {
	var final *Result
	for result, err := range results {
		if err != nil {
			return nil, err
		}
		if result != nil && result.Progress == "" {
			final = result
		}
	}
	if final == nil {
		return nil, errors.New("research: empty response")
	}
	return final, nil
}
