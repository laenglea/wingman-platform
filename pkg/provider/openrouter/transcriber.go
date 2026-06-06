package openrouter

import (
	"context"
	"encoding/base64"
	"path"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/google/uuid"
)

var _ provider.Transcriber = (*Transcriber)(nil)

type Transcriber struct {
	*Config
}

func NewTranscriber(model string, options ...Option) (*Transcriber, error) {
	return &Transcriber{
		Config: newConfig(model, options...),
	}, nil
}

func (t *Transcriber) Transcribe(ctx context.Context, input provider.File, options *provider.TranscribeOptions) (*provider.Transcription, error) {
	if options == nil {
		options = new(provider.TranscribeOptions)
	}

	body := map[string]any{
		"model": t.model,

		"input_audio": map[string]any{
			"data":   base64.StdEncoding.EncodeToString(input.Content),
			"format": detectAudioFormat(input),
		},
	}

	if options.Language != "" {
		body["language"] = options.Language
	}

	var result struct {
		Text string `json:"text"`
	}

	if err := doRequest(ctx, t.client, t.url+"/audio/transcriptions", t.token, body, &result); err != nil {
		return nil, err
	}

	return &provider.Transcription{
		ID:    uuid.NewString(),
		Model: t.model,

		Text: strings.TrimSpace(result.Text),
	}, nil
}

func detectAudioFormat(file provider.File) string {
	if file.ContentType != "" {
		// Extract subtype from MIME type (e.g. "audio/mp3" -> "mp3")
		if _, subtype, ok := strings.Cut(file.ContentType, "/"); ok {
			switch subtype {
			case "x-wav":
				return "wav"
			case "mpeg":
				return "mp3"
			case "x-aiff":
				return "aiff"
			case "mp4", "x-m4a":
				return "m4a"
			default:
				return subtype
			}
		}
	}

	if file.Name != "" {
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(file.Name), "."))

		switch ext {
		case "aif":
			return "aiff"
		case "weba":
			return "webm"
		default:
			if ext != "" {
				return ext
			}
		}
	}

	return "wav"
}
