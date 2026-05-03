package client

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type TranscriptionService struct {
	Options []RequestOption
}

func NewTranscriptionService(opts ...RequestOption) TranscriptionService {
	return TranscriptionService{
		Options: opts,
	}
}

type Transcription = provider.Transcription
type TranscribeOptions = provider.TranscribeOptions

type TranscribeRequest struct {
	TranscribeOptions

	Model string

	Name   string
	Reader io.Reader
}

func (r *TranscriptionService) New(ctx context.Context, input TranscribeRequest, opts ...RequestOption) (*Transcription, error) {
	cfg := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	if input.Language != "" {
		w.WriteField("language", input.Language)
	}

	if input.Instructions != "" {
		w.WriteField("instructions", input.Instructions)
	}

	if err := writeFormFile(w, "file", input.Name, input.Reader); err != nil {
		return nil, err
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(cfg.URL, "/v1/transcribe"), &data)
	req.Header.Set("Content-Type", w.FormDataContentType())

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := cfg.Client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	text, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return &provider.Transcription{
		Model: input.Model,
		Text:  string(text),
	}, nil
}
