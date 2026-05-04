package client

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
)

type SummaryService struct {
	Options []RequestOption
}

func NewSummaryService(opts ...RequestOption) SummaryService {
	return SummaryService{
		Options: opts,
	}
}

type Summary struct {
	Text string `json:"content"`
}

type SummaryRequest struct {
	Model string

	Text string
	URL  string

	Name   string
	Reader io.Reader
}

func (r *SummaryService) New(ctx context.Context, input SummaryRequest, opts ...RequestOption) (*Summary, error) {
	c := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	if input.Text != "" {
		w.WriteField("text", input.Text)
	}

	if input.URL != "" {
		w.WriteField("url", input.URL)
	}

	if input.Reader != nil {
		if err := writeFormFile(w, "file", input.Name, input.Reader); err != nil {
			return nil, err
		}
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint(c.URL, "/v1/summarize"), &data)
	req.Header.Set("Content-Type", w.FormDataContentType())

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

	result, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return &Summary{
		Text: string(result),
	}, nil
}
