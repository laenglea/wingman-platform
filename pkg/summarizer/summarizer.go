package summarizer

import "context"

type Provider interface {
	Summarize(ctx context.Context, text string, options *SummarizeOptions) (*Summary, error)
}

type SummarizeOptions struct {
}

type Summary struct {
	Text string
}
