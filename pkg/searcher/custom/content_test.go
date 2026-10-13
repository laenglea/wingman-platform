package custom

import (
	"context"
	"testing"

	"github.com/adrianliechti/wingman/pkg/searcher"
)

func TestFullTextCapabilityAndProtocol(t *testing.T) {
	page := "Full source page"
	rpc := &reportingClient{
		response: &CapabilitiesResponse{Content: true},
		searchClientFunc: func(_ context.Context, request *SearchRequest) (*SearchResponse, error) {
			if request.Text == nil {
				t.Fatal("full text selection was lost")
			}
			return &SearchResponse{Results: []*Result{
				{Source: "https://example.com", Excerpt: "Excerpt", Text: &page},
				{Source: "https://example.com/missing", Excerpt: "Useful excerpt"},
			}}, nil
		},
	}
	c := &Client{client: rpc}
	if err := c.loadCapabilities(); err != nil {
		t.Fatal(err)
	}
	results, err := c.Search(t.Context(), "test", &searcher.SearchOptions{Text: &searcher.TextOptions{}})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Capabilities().Content || len(results) != 2 || results[0].Excerpt != "Excerpt" || results[0].Text == nil || *results[0].Text != page || results[1].Text == nil || *results[1].Text != "" {
		t.Fatal("full text and retrieval status were not preserved")
	}
	legacy := &Client{client: searchClientFunc(func(context.Context, *SearchRequest) (*SearchResponse, error) {
		t.Fatal("legacy service must not receive unsupported full text selection")
		return nil, nil
	})}
	if err := legacy.loadCapabilities(); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Search(t.Context(), "test", &searcher.SearchOptions{Text: &searcher.TextOptions{}}); err == nil {
		t.Fatal("legacy full text capability was invented")
	}
}
