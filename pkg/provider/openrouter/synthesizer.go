package openrouter

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/google/uuid"
)

var _ provider.Synthesizer = (*Synthesizer)(nil)

type Synthesizer struct {
	*Config
}

func NewSynthesizer(model string, options ...Option) (*Synthesizer, error) {
	return &Synthesizer{
		Config: newConfig(model, options...),
	}, nil
}

func (s *Synthesizer) Synthesize(ctx context.Context, content string, options *provider.SynthesizeOptions) (*provider.Synthesis, error) {
	if options == nil {
		options = new(provider.SynthesizeOptions)
	}

	voice := "alloy"
	if options.Voice != "" {
		voice = options.Voice
	}

	format := "wav"

	if options.Format != "" {
		format = options.Format
	}

	body := map[string]any{
		"model": s.model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"modalities": []string{"text", "audio"},
		"audio": map[string]any{
			"voice":  voice,
			"format": format,
		},
		"stream": false,
	}

	var result map[string]any

	if err := doRequest(ctx, s.client, s.url+"/chat/completions", s.token, body, &result); err != nil {
		return nil, err
	}

	message, err := extractMessage(result)
	if err != nil {
		return nil, err
	}

	audio, ok := message["audio"].(map[string]any)

	if !ok {
		return nil, errors.New("no audio data in response")
	}

	audioData, ok := audio["data"].(string)

	if !ok || audioData == "" {
		return nil, errors.New("no audio data in response")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(audioData)

	if err != nil {
		return nil, err
	}

	return &provider.Synthesis{
		ID:    uuid.NewString(),
		Model: s.model,

		Content:     audioBytes,
		ContentType: "audio/" + mapAudioSubtype(format),
	}, nil
}

func mapAudioSubtype(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "mpeg"
	case "pcm16":
		return "pcm"
	default:
		return strings.ToLower(format)
	}
}
