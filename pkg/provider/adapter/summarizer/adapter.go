package summarizer

import (
	"context"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/summarizer"
	"github.com/adrianliechti/wingman/pkg/text"
)

var _ summarizer.Provider = (*Adapter)(nil)

type Adapter struct {
	completer provider.Completer
}

func FromCompleter(completer provider.Completer) *Adapter {
	return &Adapter{
		completer: completer,
	}
}

func (a *Adapter) Summarize(ctx context.Context, content string, options *summarizer.SummarizeOptions) (*summarizer.Summary, error) {
	splitter := text.NewTextSplitter()
	splitter.ChunkSize = 100000
	splitter.ChunkOverlap = 0

	var segments []string

	for _, part := range splitter.Split(content) {
		acc := provider.CompletionAccumulator{}

		for completion, err := range a.completer.Complete(ctx, []provider.Message{
			provider.UserMessage("Write a concise summary of the following section of a larger document. Use the same language as the source. Only return the summary, no other text.\n\n" + part),
		}, nil) {
			if err != nil {
				return nil, err
			}

			acc.Add(*completion)
		}

		result := acc.Result()
		segments = append(segments, result.Message.Text())
	}

	if len(segments) == 1 {
		return &summarizer.Summary{
			Text: segments[0],
		}, nil
	}

	acc := provider.CompletionAccumulator{}

	for completion, err := range a.completer.Complete(ctx, []provider.Message{
		provider.UserMessage("The following are summaries of consecutive sections of a single document. Combine them into one coherent summary of the entire document, preserving their order and removing redundancy. Use the same language as the summaries. Only return the summary, no other text.\n\n" + strings.Join(segments, "\n\n---\n\n")),
	}, nil) {
		if err != nil {
			return nil, err
		}

		acc.Add(*completion)
	}

	finalResult := acc.Result()

	result := &summarizer.Summary{
		Text: finalResult.Message.Text(),
	}

	return result, nil
}
