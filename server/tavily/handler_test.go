package tavily_test

import (
	"strings"
	"testing"
	"time"

	"github.com/adrianliechti/wingman/test/webtest"
)

func TestSearchAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		const dialect = "tavily"
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
			if hasScore != (backend == "tavily") {
				t.Fatal("score invented or discarded")
			}
			if got["images"] == nil || got["response_time"] == nil || got["request_id"] == nil {
				t.Fatal("missing SDK fields")
			}
		})
	}
}

func TestSearchUsesPublishedDefaultResultCount(t *testing.T) {
	s, p := webtest.NewServer(t, false)
	webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"query":"test"}`, nil), 200)
	if p["tavily"].Options.Limit == nil || *p["tavily"].Options.Limit != 10 {
		t.Fatal("default max_results must match Tavily's published schema")
	}
}

func TestSearchFiltersAndDates(t *testing.T) {
	for _, tc := range []struct {
		path, body, backend, category string
		inclusive                     bool
	}{
		{"/v1/tavily/search", `{"model":"exa","query":"test","topic":"finance","max_results":2,"start_date":"2026-10-01","end_date":"2026-10-02","include_domains":["example.com"],"exclude_domains":["spam.com"]}`, "exa", "financial report", true},
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
	s, p := webtest.NewServer(t, false)
	webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"query":"test","country":"switzerland","time_range":"week"}`, nil), 200)
	if p["tavily"].Options.Location != "CH" || p["tavily"].Options.Since == nil {
		t.Fatal("country or time range lost")
	}
	got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"query":"test","max_results":0}`, nil), 200)
	if len(got["results"].([]any)) != 0 || p["tavily"].Searches != 1 {
		t.Fatal("zero results called provider")
	}
}

func TestBatchExtractionAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		const dialect = "tavily"
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
	got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/extract", `{"urls":"https://example.com/missing"}`, nil), 200)
	if len(got["results"].([]any)) != 0 || len(got["failed_results"].([]any)) != 1 {
		t.Fatal("empty arrays not represented")
	}
}

func TestCredentials(t *testing.T) {
	s, _ := webtest.NewServer(t, false)
	webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", `{"query":"test","api_key":"wingman-test-key"}`, map[string]string{}), 200)
	for _, path := range []string{"/v1/tavily/search"} {
		got := webtest.Response(t, webtest.Send(t, s, path, `{"query":"test","api_key":"wingman-test-key"}`, map[string]string{"Authorization": "Bearer wrong", "x-api-key": "wingman-test-key"}), 401)
		if strings.Contains(path, "tavily") && got["detail"] == nil {
			t.Fatal("wrong Tavily authentication error")
		}
		if strings.Contains(path, "exa") && got["error"] == nil {
			t.Fatal("wrong Exa authentication error")
		}
	}
}

func TestRejectsInvalidSupportedFieldsBeforeRetrieval(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/v1/tavily/search", `{"query":"test","include_raw_content":{}}`},
		{"/v1/tavily/search", `{"query":"test","max_results":21}`},
		{"/v1/tavily/search", `{"query":"test","max_results":1.5}`},
		{"/v1/tavily/search", `{"query":"test","model":"plain","topic":"news"}`},
		{"/v1/tavily/search", `{"query":"test","model":"plain","time_range":"day"}`},
		{"/v1/tavily/search", `{"query":"test","start_date":"2026-10-03","end_date":"2026-10-01"}`},
		{"/v1/tavily/search", `{"query":"test","country":"not-a-country"}`},
		{"/v1/tavily/search", `{"query":"test","include_domains":["https://example.com"]}`},
		{"/v1/tavily/extract", `{"urls":["file:///etc/passwd"]}`},
		{"/v1/tavily/extract", `{"urls":["https://user:pass@example.com"]}`},
		{"/v1/tavily/extract", `{"urls":["https://example.com"],"timeout":0}`},
		{"/v1/tavily/search", `null`},
		{"/v1/tavily/search", `{"query":"test"} {}`},
		{"/v1/tavily/search", `{"query":" "}`},
		{"/v1/tavily/search?model=exa", `{"model":"tavily","query":"test"}`},
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
		{"/v1/tavily/search", `{"query":"test"}`},
		{"/v1/tavily/extract", `{"urls":"https://example.com"}`},
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
