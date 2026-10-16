package tavily_test

import (
	"testing"

	"github.com/adrianliechti/wingman/pkg/searcher"
	"github.com/adrianliechti/wingman/test/webtest"
)

func TestOptionalTextAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		for _, selector := range []string{"true", `"markdown"`, `"text"`} {
			s, providers := webtest.NewServer(t, false)
			p := providers[backend]
			page, empty := "Native full page", ""
			p.Caps.Content = backend != "plain"
			p.Results = []searcher.Result{
				{Source: "https://example.com", Excerpt: "Evidence"},
				{Source: "https://example.com/missing", Excerpt: "Available excerpt"},
			}
			if p.Caps.Content {
				p.Results[0].Text = &page
				p.Results[1].Text = &empty
			}
			got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"model":"`+backend+`","query":"test","include_raw_content":`+selector+`}`, nil), 200)
			hits := got["results"].([]any)
			if len(hits) != 2 || hits[0].(map[string]any)["content"] != "Evidence" || hits[1].(map[string]any)["content"] != "Available excerpt" || hits[1].(map[string]any)["raw_content"] != nil {
				t.Fatal("excerpts or partial failure were lost")
			}
			wantPage, wantFetches := page, 0
			if !p.Caps.Content {
				wantPage, wantFetches = "Grüezi 世界 — full page", 2
			}
			if hits[0].(map[string]any)["raw_content"] != wantPage || p.Scrapes != wantFetches || (p.Options.Text != nil) != p.Caps.Content {
				t.Fatalf("%s: incorrect native retrieval or fallback: options=%+v fetches=%d", backend, p.Options, p.Scrapes)
			}
		}
	}
}

func TestNativeUnavailableTextDoesNotTriggerFreshFetch(t *testing.T) {
	s, providers := webtest.NewServer(t, false)
	p := providers["exa"]
	p.Caps.Content = true
	// A native provider that omits text has already attempted retrieval.
	got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"model":"exa","query":"test","include_raw_content":true}`, nil), 200)
	hit := got["results"].([]any)[0].(map[string]any)
	if p.Scrapes != 0 || hit["raw_content"] != nil || hit["content"] != "Evidence" {
		t.Fatal("unavailable text triggered a fresh fetch or lost the excerpt")
	}
}
