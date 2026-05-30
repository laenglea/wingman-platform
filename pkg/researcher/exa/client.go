package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/adrianliechti/wingman/pkg/researcher"
)

var _ researcher.Provider = &Client{}

type Client struct {
	url    string
	token  string
	client *http.Client
}

func New(token string, options ...Option) (*Client, error) {
	c := &Client{
		url: "https://api.exa.ai",

		token:  token,
		client: http.DefaultClient,
	}

	for _, option := range options {
		option(c)
	}

	if c.token == "" {
		return nil, errors.New("invalid token")
	}

	return c, nil
}

func (c *Client) Research(ctx context.Context, instructions string, options *researcher.ResearchOptions) (*researcher.Result, error) {
	if options == nil {
		options = new(researcher.ResearchOptions)
	}

	request := &SearchRequest{
		Query: instructions,
		Type:  "deep-reasoning",

		OutputSchema: &OutputSchema{
			Type:        "text",
			Description: "A comprehensive, well-structured research report that answers the question.",
		},
	}

	body, _ := json.Marshal(request)

	url := strings.TrimRight(c.url, "/") + "/search"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.token)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exa: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var data SearchResponse

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Output == nil {
		return nil, errors.New("exa: no research output returned")
	}

	content := strings.TrimSpace(decodeContent(data.Output.Content))
	content = appendSources(content, collectSources(data.Output.Grounding))

	return &researcher.Result{
		Content: content,
	}, nil
}

type source struct {
	title string
	url   string
}

// decodeContent returns the synthesized text. With a "text" outputSchema the
// content is a JSON string; fall back to the raw bytes for any other shape.
func decodeContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string

	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	return string(raw)
}

func collectSources(grounding []Grounding) []source {
	var sources []source
	seen := map[string]struct{}{}

	for _, g := range grounding {
		for _, c := range g.Citations {
			if c.URL == "" {
				continue
			}

			if _, ok := seen[c.URL]; ok {
				continue
			}

			seen[c.URL] = struct{}{}
			sources = append(sources, source{title: c.Title, url: c.URL})
		}
	}

	return sources
}

func appendSources(content string, sources []source) string {
	if len(sources) == 0 {
		return content
	}

	var b strings.Builder

	if content != "" {
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	b.WriteString("Sources:")

	for _, source := range sources {
		b.WriteString("\n- ")

		if source.title != "" {
			b.WriteString(source.title)
			b.WriteString(": ")
		}

		b.WriteString(source.url)
	}

	return b.String()
}
