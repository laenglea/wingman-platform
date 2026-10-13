package exa_test

import (
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/pkg/searcher"
	"github.com/adrianliechti/wingman/test/webtest"
)

func TestSearchAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		const dialect = "exa"
		t.Run(dialect+"/"+backend, func(t *testing.T) {
			s, providers := webtest.NewServer(t, false)
			body := `{"query":"test","model":"` + backend + `"}`
			got := webtest.Response(t, webtest.Send(t, s, "/v1/"+dialect+"/search", body, nil), 200)
			results := got["results"].([]any)
			if len(results) != 1 || providers[backend].Searches != 1 {
				t.Fatal("backend was not selected")
			}
			hit := results[0].(map[string]any)
			if hit["url"] != "https://example.com" {
				t.Fatal("lost source")
			}
			_, hasScore := hit["score"]
			if hasScore {
				t.Fatal("score invented or discarded")
			}
		})
	}
}

func TestSearchFiltersAndDates(t *testing.T) {
	for _, tc := range []struct {
		path, body, backend, category string
		inclusive                     bool
	}{
		{"/v1/exa/search", `{"model":"tavily","query":"test","category":"financial report","numResults":2,"startPublishedDate":"2026-10-01T00:00:00Z","endPublishedDate":"2026-10-02T00:00:00Z","includeDomains":["example.com"],"excludeDomains":["spam.com"]}`, "tavily", "finance", false},
	} {
		s, p := webtest.NewServer(t, false)
		webtest.Response(t, webtest.Send(t, s, tc.path, tc.body, nil), 200)
		opts := p[tc.backend].Options
		wantUntil := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		if tc.inclusive {
			wantUntil = wantUntil.AddDate(0, 0, 1)
		}
		if opts.Category != tc.category || *opts.Limit != 2 || opts.Since == nil || opts.Until == nil || !opts.Until.Equal(wantUntil) || opts.Include[0] != "example.com" || opts.Exclude[0] != "spam.com" {
			t.Fatalf("filters: %+v", opts)
		}
	}
}

func TestBatchExtractionAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		const dialect = "exa"
		t.Run(dialect+"/"+backend, func(t *testing.T) {
			s, p := webtest.NewServer(t, false)
			endpoint := "extract"
			if dialect == "exa" {
				endpoint = "contents"
			}
			body := `{"model":"` + backend + `","urls":["https://example.com","https://example.com/missing"]}`
			got := webtest.Response(t, webtest.Send(t, s, "/v1/"+dialect+"/"+endpoint, body, nil), 200)
			if len(got["results"].([]any)) != 1 || p[backend].Scrapes != 2 {
				t.Fatal("incorrect partial success")
			}
			if dialect == "tavily" {
				if len(got["failed_results"].([]any)) != 1 {
					t.Fatal("missing failure")
				}
			} else if len(got["statuses"].([]any)) != 2 || got["statuses"].([]any)[1].(map[string]any)["status"] != "error" {
				t.Fatal("missing Exa status")
			}
		})
	}
	s, _ := webtest.NewServer(t, false)
	got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/contents", `{"ids":["https://example.com"],"text":{"maxCharacters":8}}`, nil), 200)
	if got["results"].([]any)[0].(map[string]any)["text"] != "Grüezi 世" {
		t.Fatal("character limit corrupted Unicode")
	}
}

func TestSearchDoesNotFetchUnrequestedContents(t *testing.T) {
	for _, body := range []string{
		`{"query":"test","contents":{}}`,
		`{"query":"test","contents":{"highlights":false,"summary":false}}`,
		`{"query":"test","contents":{"text":false}}`,
	} {
		s, p := webtest.NewServer(t, false)
		got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", body, nil), 200)
		if p["tavily"].Scrapes != 0 {
			t.Fatal("metadata search made an unrequested fetch")
		}
		if _, ok := got["results"].([]any)[0].(map[string]any)["text"]; ok {
			t.Fatal("metadata search included unrequested text")
		}
	}
}

func TestExaSearchAndContents(t *testing.T) {
	s, p := webtest.NewServer(t, false)
	got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", `{"model":"tavily","query":"test","contents":{"text":{"maxCharacters":6}}}`, nil), 200)
	if p["tavily"].Searches != 1 || p["tavily"].Scrapes != 1 || got["results"].([]any)[0].(map[string]any)["text"] != "Grüezi" {
		t.Fatal("search and contents not composed")
	}
}

func TestSearchReusesFullTextAndOnlyFetchesUnattemptedResults(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fallback bool
	}{{"all retrieved", false}, {"mixed providers", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := webtest.NewServer(t, false)
			full, empty := "Grüezi 世界 — previously retrieved full page", ""
			p["exa"].Results = []searcher.Result{
				{Source: "https://example.com/full", Excerpt: "short excerpt", Text: &full},
				{Source: "https://example.com/empty", Text: &empty},
			}
			if tc.fallback {
				p["exa"].Results = append(p["exa"].Results, searcher.Result{Source: "https://example.com/fallback", Excerpt: "another excerpt"})
			}
			got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", `{"model":"exa","query":"test","contents":{"text":{"maxCharacters":8}}}`, nil), 200)
			hits := got["results"].([]any)
			if p["exa"].Searches != 1 || hits[0].(map[string]any)["text"] != "Grüezi 世" {
				t.Fatal("did not reuse complete search text")
			}
			if _, exists := hits[1].(map[string]any)["text"]; exists {
				t.Fatal("unavailable text was invented")
			}
			if got["statuses"].([]any)[1].(map[string]any)["status"] != "error" {
				t.Fatal("failed search text retrieval was hidden")
			}
			want := 0
			if tc.fallback {
				want = 1
				if hits[2].(map[string]any)["text"] != "Grüezi 世" {
					t.Fatal("fallback text was not returned in the correct position")
				}
			}
			if p["exa"].Scrapes != want {
				t.Fatalf("made %d scraper calls, want %d", p["exa"].Scrapes, want)
			}
		})
	}
}

func TestCredentials(t *testing.T) {
	s, _ := webtest.NewServer(t, false)
	webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", `{"query":"test"}`, map[string]string{"x-api-key": "wingman-test-key"}), 200)
	for _, path := range []string{"/v1/exa/search"} {
		got := webtest.Response(t, webtest.Send(t, s, path, `{"query":"test","api_key":"wingman-test-key"}`, map[string]string{"Authorization": "Bearer wrong", "x-api-key": "wingman-test-key"}), 401)
		if strings.Contains(path, "tavily") && got["detail"] == nil {
			t.Fatal("wrong Tavily authentication error")
		}
		if strings.Contains(path, "exa") && got["error"] == nil {
			t.Fatal("wrong Exa authentication error")
		}
	}
}

func TestRejectsUnsupportedOrInvalidRequestsBeforeRetrieval(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/v1/exa/search", `{"query":"test","type":"neural"}`},
		{"/v1/exa/search", `{"query":"test","contents":{"text":true,"unknown":false}}`},
		{"/v1/exa/search", `{"query":"test","contents":{"highlights":{"query":"different query"}}}`},
		{"/v1/exa/search", `{"query":"test","contents":{"text":{"maxCharacters":1000001}}}`},
		{"/v1/exa/search", `{"query":"test","numResults":21}`},
		{"/v1/exa/search", `{"query":"test","model":"exa","category":"company"}`},
		{"/v1/exa/contents", `{"urls":["https://example.com"],"text":false}`},
		{"/v1/exa/contents", `{"urls":["https://example.com"],"text":{"maxCharacters":0}}`},
		{"/v1/exa/contents", `{"urls":[],"text":true}`},
		{"/v1/exa/contents", `{"urls":["https://example.com"],"ids":["https://example.com"]}`},
	} {
		t.Run(tc.body, func(t *testing.T) {
			s, providers := webtest.NewServer(t, false)
			webtest.Response(t, webtest.Send(t, s, tc.path, tc.body, nil), 400)
			for _, p := range providers {
				if p.Searches != 0 || p.Scrapes != 0 {
					t.Fatal("invalid request reached provider")
				}
			}
		})
	}
}

func TestPolicyDenialStopsRetrieval(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/v1/exa/search", `{"query":"test","contents":{"text":true}}`},
		{"/v1/exa/contents", `{"urls":["https://example.com"]}`},
	} {
		s, providers := webtest.NewServer(t, true)
		webtest.Response(t, webtest.Send(t, s, tc.path, tc.body, nil), 404)
		for _, p := range providers {
			if p.Searches != 0 || p.Scrapes != 0 {
				t.Fatal("denied request reached provider")
			}
		}
	}
}
