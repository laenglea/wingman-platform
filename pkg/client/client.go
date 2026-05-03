package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
)

type Client struct {
	Models ModelService

	Reranks     RerankService
	Embeddings  EmbeddingService
	Completions CompletionService

	Syntheses      SynthesisService
	Renderings     RenderingService
	Transcriptions TranscriptionService

	Scrapes    ScrapeService
	Searches   SearchService
	Researches ResearchService

	Segments    SegmentService
	Extractions ExtractionService

	Summaries    SummaryService
	Translations TranslationService
}

func New(url string, opts ...RequestOption) *Client {
	opts = append(opts, WithURL(url))

	return &Client{
		Models: NewModelService(opts...),

		Reranks:     NewRerankService(opts...),
		Embeddings:  NewEmbeddingService(opts...),
		Completions: NewCompletionService(opts...),

		Syntheses:      NewSynthesisService(opts...),
		Renderings:     NewRenderingService(opts...),
		Transcriptions: NewTranscriptionService(opts...),

		Scrapes:    NewScrapeService(opts...),
		Searches:   NewSearchService(opts...),
		Researches: NewResearchService(opts...),

		Segments:    NewSegmentService(opts...),
		Extractions: NewExtractionService(opts...),

		Summaries:    NewSummaryService(opts...),
		Translations: NewTranslationService(opts...),
	}
}

func newRequestConfig(opts ...RequestOption) *RequestConfig {
	c := &RequestConfig{
		Client: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func Ptr[T any](v T) *T {
	return &v
}

func endpoint(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func writeFormFile(w *multipart.Writer, field, name string, r io.Reader) error {
	contentType := mime.TypeByExtension(filepath.Ext(name))

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     field,
		"filename": name,
	}))
	h.Set("Content-Type", contentType)

	part, err := w.CreatePart(h)

	if err != nil {
		return err
	}

	_, err = io.Copy(part, r)
	return err
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	text := strings.TrimSpace(string(body))

	if text == "" {
		return responseError{Status: resp.Status, StatusCode: resp.StatusCode}
	}

	if err := parseResponseError(resp, body, text); err != nil {
		return err
	}

	return responseError{Status: resp.Status, StatusCode: resp.StatusCode, Message: text}
}

type responseError struct {
	Status     string
	StatusCode int

	Message string
	Type    string
	Code    string
}

func (e responseError) Error() string {
	var parts []string

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Type != "" {
		parts = append(parts, "type="+e.Type)
	}

	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}

	if len(parts) == 0 {
		return e.Status
	}

	return fmt.Sprintf("%s: %s", e.Status, strings.Join(parts, " "))
}

func parseResponseError(resp *http.Response, body []byte, fallback string) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var data any

	if err := dec.Decode(&data); err != nil {
		return nil
	}

	result := responseError{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	if msg, typ, code := extractErrorFields(data); msg != "" || typ != "" || code != "" {
		result.Message = msg
		result.Type = typ
		result.Code = code
		return result
	}

	result.Message = fallback
	return result
}

func extractErrorFields(data any) (message, typ, code string) {
	obj, ok := data.(map[string]any)

	if !ok {
		return "", "", ""
	}

	message = stringField(obj, "message")
	typ = stringField(obj, "type")
	code = stringField(obj, "code")

	if detail := stringField(obj, "detail"); message == "" && detail != "" {
		message = detail
	}

	if errValue, ok := obj["error"]; ok {
		switch err := errValue.(type) {
		case string:
			if message == "" {
				message = err
			}

		case map[string]any:
			if msg := stringField(err, "message"); msg != "" {
				message = msg
			}

			if val := stringField(err, "type"); val != "" {
				typ = val
			}

			if val := stringField(err, "code"); val != "" {
				code = val
			}
		}
	}

	return message, typ, code
}

func stringField(obj map[string]any, key string) string {
	switch value := obj[key].(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return fmt.Sprintf("%g", value)
	}

	return ""
}
