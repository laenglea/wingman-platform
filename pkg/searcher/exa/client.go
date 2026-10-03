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
	"time"

	"github.com/adrianliechti/wingman/pkg/searcher"
)

var _ searcher.Provider = &Client{}

type Client struct {
	token  string
	client *http.Client

	mode string

	category string
	location string
}

func New(token string, options ...Option) (*Client, error) {
	c := &Client{
		token:  token,
		client: http.DefaultClient,

		mode: "fast",
	}

	for _, option := range options {
		option(c)
	}

	if c.token == "" {
		return nil, errors.New("invalid token")
	}

	// Search stays retrieval-only. Synthesis belongs to Wingman's own model.
	switch c.mode {
	case "fast", "instant", "auto":
	default:
		return nil, fmt.Errorf("exa search: mode %q is not a retrieval-only search mode", c.mode)
	}

	return c, nil
}

func (c *Client) Search(ctx context.Context, query string, options *searcher.SearchOptions) ([]searcher.Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("exa search: query is required")
	}

	// Do not mutate caller-owned options (they may be reused concurrently).
	settings := searcher.SearchOptions{}
	if options != nil {
		settings = *options
	}
	if settings.Limit != nil && (*settings.Limit < 1 || *settings.Limit > 100) {
		return nil, errors.New("exa search: limit must be between 1 and 100")
	}
	if settings.Category == "" {
		settings.Category = c.category
	}
	if settings.Location == "" {
		settings.Location = c.location
	}

	request := &SearchRequest{
		Query: query,

		Location: settings.Location,

		NumResults: settings.Limit,

		IncludeDomains: settings.Include,
		ExcludeDomains: settings.Exclude,

		Contents: &SearchContents{
			// Plain source text only: no Exa highlights, summaries or outputSchema.
			// Select a query-relevant excerpt locally after retrieval.
			Text: true,
		},
	}

	request.Category = settings.Category

	if c.mode != "" {
		request.Type = c.mode
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("exa search: HTTP %d: %s", resp.StatusCode, body)
	}

	var data SearchResponse

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []searcher.Result

	for _, r := range data.Results {
		result := searcher.Result{
			Source: r.URL,

			Title:   r.Title,
			Content: searchExcerpt(r.Text, query),
		}

		if t, err := time.Parse(time.RFC3339, r.PublishedDate); err == nil {
			result.Timestamp = &t
			result.Metadata = map[string]string{"published": t.Format(time.RFC3339)}
		}

		results = append(results, result)
	}

	return results, nil
}

const (
	CategoryCompany         = "company"
	CategoryPeople          = "people"
	CategoryNews            = "news"
	CategoryResearchPaper   = "research paper"
	CategoryPersonalSite    = "personal site"
	CategoryFinancialReport = "financial report"
)

func (c *Client) Categories() []searcher.Category {
	return []searcher.Category{
		{Name: CategoryCompany, Description: "Specific companies or organizations (e.g. SaaS vendors, public companies). Note: domain exclusions and date filters are not supported in this category."},
		{Name: CategoryPeople, Description: "Specific people or profile pages (e.g. LinkedIn-style biographies). Note: domain exclusions and date filters are not supported in this category."},
		{Name: CategoryNews, Description: "News articles and current-events coverage from media outlets."},
		{Name: CategoryResearchPaper, Description: "Academic papers and peer-reviewed research publications."},
		{Name: CategoryPersonalSite, Description: "Personal websites, blogs, and homepages."},
		{Name: CategoryFinancialReport, Description: "Earnings releases, 10-K/10-Q filings, and other financial reports."},
	}
}
