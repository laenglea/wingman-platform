package tool

import (
	"context"
	"iter"
)

// Streamer is an optional interface for tools that report intermediate progress.
type Streamer interface {
	ExecuteStream(ctx context.Context, name string, parameters map[string]any) iter.Seq2[*Execution, error]
}

type Execution struct {
	// Progress describes an intermediate step; Result is the final tool value.
	Progress string
	Result   any
}

// Collect consumes a stream and returns its final result, passing intermediate
// progress to onProgress when it is non-nil.
func Collect(seq iter.Seq2[*Execution, error], onProgress func(string)) (any, error) {
	var result any
	for execution, err := range seq {
		if err != nil {
			return nil, err
		}
		if execution == nil {
			continue
		}
		if execution.Progress == "" {
			result = execution.Result
		} else if onProgress != nil {
			onProgress(execution.Progress)
		}
	}
	return result, nil
}

// ExecuteStream uses the provider's stream when available, or yields its result.
func ExecuteStream(ctx context.Context, p Provider, name string, parameters map[string]any) iter.Seq2[*Execution, error] {
	if p, ok := p.(Streamer); ok {
		return p.ExecuteStream(ctx, name, parameters)
	}
	return func(yield func(*Execution, error) bool) {
		result, err := p.Execute(ctx, name, parameters)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(&Execution{Result: result}, nil)
	}
}
