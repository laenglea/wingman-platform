package openai

import (
	"bytes"
	"context"
	"io"
	"iter"

	"github.com/adrianliechti/wingman/pkg/provider"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
)

var _ provider.Synthesizer = (*Synthesizer)(nil)

type Synthesizer struct {
	*Config
	speech openai.AudioSpeechService
}

func NewSynthesizer(url, model string, options ...Option) (*Synthesizer, error) {
	cfg := &Config{
		url:   url,
		model: model,
	}

	for _, option := range options {
		option(cfg)
	}

	return &Synthesizer{
		Config: cfg,
		speech: openai.NewAudioSpeechService(cfg.AzureOptions()...),
	}, nil
}

func (s *Synthesizer) Synthesize(ctx context.Context, content string, options *provider.SynthesizeOptions) iter.Seq2[*provider.Synthesis, error] {
	return func(yield func(*provider.Synthesis, error) bool) {
		if options == nil {
			options = new(provider.SynthesizeOptions)
		}

		params := openai.AudioSpeechNewParams{
			Model: s.model,
			Input: content,

			Voice: openai.AudioSpeechNewParamsVoiceUnion{
				OfString: openai.String(string(openai.AudioSpeechNewParamsVoiceString2Alloy)),
			},
		}

		if options.Voice != "" {
			params.Voice = openai.AudioSpeechNewParamsVoiceUnion{
				OfString: openai.String(options.Voice),
			}
		}

		if options.Speed != nil {
			params.Speed = openai.Float(float64(*options.Speed))
		}

		if options.Format != "" {
			params.ResponseFormat = openai.AudioSpeechNewParamsResponseFormat(options.Format)
		}

		result, err := s.speech.New(ctx, params)

		if err != nil {
			yield(nil, convertError(err))
			return
		}

		defer result.Body.Close()

		id := uuid.NewString()

		contentType := "audio/mpeg"

		if ct := result.Header.Get("Content-Type"); ct != "" {
			contentType = ct
		}

		buffer := make([]byte, 32*1024)

		for {
			n, err := result.Body.Read(buffer)

			if n > 0 {
				chunk := provider.Synthesis{
					ID:    id,
					Model: s.model,

					Content:     bytes.Clone(buffer[:n]),
					ContentType: contentType,
				}

				if !yield(&chunk, nil) {
					return
				}
			}

			if err != nil {
				if err != io.EOF {
					yield(nil, err)
				}

				return
			}
		}
	}
}
