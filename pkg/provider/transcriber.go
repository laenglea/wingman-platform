package provider

import (
	"context"
	"iter"
	"strings"
)

// Transcriber streams a transcription as deltas. Backends without incremental
// transcription yield a single complete result.
type Transcriber interface {
	Transcribe(ctx context.Context, input File, options *TranscribeOptions) iter.Seq2[*Transcription, error]
}

type TranscribeOptions struct {
	Languages []string

	Keywords []string

	Instructions string

	Timestamps bool

	Diarize  bool
	Speakers []TranscribeSpeaker

	ChunkingStrategy string
}

type TranscribeSpeaker struct {
	Name string

	Reference File
}

type Transcription struct {
	ID    string
	Model string

	Text string

	Language  string
	Languages []string

	Duration float64

	Segments []TranscriptionSegment
	Words    []TranscriptionWord
}

type TranscriptionSegment struct {
	ID string

	Speaker string

	Start float64
	End   float64

	Text string
}

type TranscriptionWord struct {
	Word string

	Start float64
	End   float64
}

type TranscriptionAccumulator struct {
	id    string
	model string

	text strings.Builder

	language  string
	languages []string

	duration float64

	segments []TranscriptionSegment
	words    []TranscriptionWord
}

func (a *TranscriptionAccumulator) Add(t Transcription) {
	if t.ID != "" {
		a.id = t.ID
	}

	if t.Model != "" {
		a.model = t.Model
	}

	a.text.WriteString(t.Text)

	if t.Language != "" {
		a.language = t.Language
	}

	if len(t.Languages) > 0 {
		a.languages = t.Languages
	}

	if t.Duration > 0 {
		a.duration = t.Duration
	}

	a.segments = append(a.segments, t.Segments...)
	a.words = append(a.words, t.Words...)
}

func (a *TranscriptionAccumulator) Result() Transcription {
	return Transcription{
		ID:    a.id,
		Model: a.model,

		Text: a.text.String(),

		Language:  a.language,
		Languages: a.languages,

		Duration: a.duration,

		Segments: a.segments,
		Words:    a.words,
	}
}
