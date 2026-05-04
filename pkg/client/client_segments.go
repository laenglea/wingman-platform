package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/adrianliechti/wingman/server/api"
)

type SegmentService struct {
	Options []RequestOption
}

func NewSegmentService(opts ...RequestOption) SegmentService {
	return SegmentService{
		Options: opts,
	}
}

type Segment = api.Segment

type SegmentRequest struct {
	Model string

	Name   string
	Reader io.Reader

	SegmentLength  *int
	SegmentOverlap *int
}

func (r *SegmentService) New(ctx context.Context, input SegmentRequest, opts ...RequestOption) ([]Segment, error) {
	c := newRequestConfig(append(r.Options, opts...)...)

	var data bytes.Buffer
	w := multipart.NewWriter(&data)

	if input.Model != "" {
		w.WriteField("model", input.Model)
	}

	if err := writeFormFile(w, "file", input.Name, input.Reader); err != nil {
		return nil, err
	}

	if input.SegmentLength != nil {
		w.WriteField("segment_length", fmt.Sprintf("%d", *input.SegmentLength))
	}

	if input.SegmentOverlap != nil {
		w.WriteField("segment_overlap", fmt.Sprintf("%d", *input.SegmentOverlap))
	}

	w.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint(c.URL, "/v1/segment"), &data)
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

	var result []Segment

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
