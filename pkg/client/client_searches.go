package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/adrianliechti/wingman/server/api"
)

type SearchService struct {
	Options []RequestOption
}

func NewSearchService(opts ...RequestOption) SearchService {
	return SearchService{
		Options: opts,
	}
}

type SearchResult = api.SearchResult

type SearchRequest struct {
	Query string

	Model string
	Limit *int

	Category string
	Location string

	Include []string
	Exclude []string
}

func (r *SearchService) New(ctx context.Context, input SearchRequest, opts ...RequestOption) ([]SearchResult, error) {
	c := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	w.WriteField("query", input.Query)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	if input.Limit != nil {
		w.WriteField("limit", fmt.Sprintf("%d", *input.Limit))
	}

	if input.Category != "" {
		w.WriteField("category", input.Category)
	}

	if input.Location != "" {
		w.WriteField("location", input.Location)
	}

	for _, domain := range input.Include {
		w.WriteField("domain", domain)
	}

	for _, domain := range input.Exclude {
		w.WriteField("domain", "!"+domain)
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint(c.URL, "/v1/search"), &data)
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

	var result []SearchResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
