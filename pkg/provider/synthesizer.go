package provider

import (
	"bytes"
	"context"
	"iter"
)

// Synthesizer streams synthesized audio as chunks. Backends without
// incremental synthesis yield a single complete result.
type Synthesizer interface {
	Synthesize(ctx context.Context, input string, options *SynthesizeOptions) iter.Seq2[*Synthesis, error]
}

type SynthesizeOptions struct {
	Voice string
	Speed *float32

	Instructions string

	Format string
}

type Synthesis struct {
	ID    string
	Model string

	Content     []byte
	ContentType string
}

type SynthesisAccumulator struct {
	id    string
	model string

	contentType string

	content bytes.Buffer
}

func (a *SynthesisAccumulator) Add(s Synthesis) {
	if s.ID != "" {
		a.id = s.ID
	}

	if s.Model != "" {
		a.model = s.Model
	}

	if s.ContentType != "" {
		a.contentType = s.ContentType
	}

	a.content.Write(s.Content)
}

func (a *SynthesisAccumulator) Result() Synthesis {
	return Synthesis{
		ID:    a.id,
		Model: a.model,

		Content:     a.content.Bytes(),
		ContentType: a.contentType,
	}
}
