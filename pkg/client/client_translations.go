package client

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
)

type TranslationService struct {
	Options []RequestOption
}

func NewTranslationService(opts ...RequestOption) TranslationService {
	return TranslationService{
		Options: opts,
	}
}

type Translation struct {
	Content     []byte
	ContentType string
}

func (t *Translation) Text() string {
	return string(t.Content)
}

type TranslationRequest struct {
	Model    string
	Language string

	Name   string
	Reader io.Reader
}

func (r *TranslationService) New(ctx context.Context, input TranslationRequest, opts ...RequestOption) (*Translation, error) {
	c := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	if input.Language != "" {
		w.WriteField("language", input.Language)
	}

	if err := writeFormFile(w, "file", input.Name, input.Reader); err != nil {
		return nil, err
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(c.URL, "/v1/translate"), &data)
	req.Header.Set("Content-Type", w.FormDataContentType())

	req.Header.Set("Accept", "application/octet-stream")

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.Client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	content, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return &Translation{
		Content:     content,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}
