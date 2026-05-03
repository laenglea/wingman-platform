package client

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"

	"github.com/adrianliechti/wingman/server/api"
)

type ResearchService struct {
	Options []RequestOption
}

func NewResearchService(opts ...RequestOption) ResearchService {
	return ResearchService{
		Options: opts,
	}
}

type ResearchResult = api.ResearchResult

type ResearchRequest struct {
	Input string
	Model string
}

func (r *ResearchService) New(ctx context.Context, input ResearchRequest, opts ...RequestOption) (*ResearchResult, error) {
	c := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	w.WriteField("input", input.Input)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(c.URL, "/v1/research"), &data)
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

	var result ResearchResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
