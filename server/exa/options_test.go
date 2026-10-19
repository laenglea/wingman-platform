package exa_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/webtest"
)

func TestTuningHintsAcrossProviders(t *testing.T) {
	for _, backend := range []string{"exa", "tavily", "plain"} {
		for _, mode := range []string{"auto", "lite", "fast", "instant", "neural", "keyword", "deep-lite", "deep", "future-mode"} {
			t.Run(backend+"/"+mode, func(t *testing.T) {
				s, providers := webtest.NewServer(t, false)
				body := `{"model":"` + backend + `","query":"test","numResults":2,
					"includeDomains":["example.com"],"type":"` + mode + `",
					"useAutoprompt":true,"userIdentifier":"test-user","stream":true,"objective":"Find supporting evidence",
					"outputSchema":{"type":"object"},"future_option":{"enabled":true},
					"startCrawlDate":"2026-01-01","endCrawlDate":"2026-10-01",
					"contents":{"livecrawlTimeout":15000,"summary":true,"maxAgeHours":-1,"future_option":[],
						"text":{"maxCharacters":8,"verbosity":"full","includeHtmlTags":true,"future_option":{"enabled":true}},
						"highlights":{"maxCharacters":4,"verbosity":"high","numSentences":3,"highlightsPerUrl":2,"query":"other query","future_option":true}}}`
				got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", body, nil), 200)
				hit := got["results"].([]any)[0].(map[string]any)
				p := providers[backend]
				if p.Searches != 1 || p.Scrapes != 1 || p.Query != "test" || *p.Options.Limit != 2 || p.Options.Include[0] != "example.com" {
					t.Fatalf("hints changed retrieval: %+v", p.Options)
				}
				if hit["text"] != "Grüezi 世" || hit["highlights"].([]any)[0] != "Evid" {
					t.Fatal("hints lost the requested content or character limits")
				}
				if hit["summary"] != nil || got["output"] != nil {
					t.Fatal("unsupported output was fabricated")
				}
			})
		}
	}
}

func TestLivecrawlTimeoutDoesNotEnableContentRetrieval(t *testing.T) {
	for _, timeout := range []string{"15000", "null", `"future-value"`, `{"future":true}`} {
		s, providers := webtest.NewServer(t, false)
		got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", `{"query":"test","contents":{"livecrawlTimeout":`+timeout+`}}`, nil), 200)
		hit := got["results"].([]any)[0].(map[string]any)
		if providers["tavily"].Scrapes != 0 || providers["tavily"].Options.Text != nil || hit["text"] != nil || hit["highlights"] != nil {
			t.Fatal("timeout hint enabled unrequested content")
		}
	}
}

func TestContentsAcceptsTimeoutAndRenderingHints(t *testing.T) {
	for _, backend := range []string{"exa", "tavily", "plain"} {
		s, providers := webtest.NewServer(t, false)
		body := `{"model":"` + backend + `","urls":["https://example.com","https://example.com/missing"],
			"livecrawlTimeout":15000,"userIdentifier":"test-user","summary":true,"highlights":true,
			"snapshotAsOf":"2026-01-01","future_option":[1,2],
			"text":{"verbosity":"full","maxCharacters":8,"future_option":{"enabled":true}}}`
		got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/contents", body, nil), 200)
		if providers[backend].Scrapes != 2 || len(got["results"].([]any)) != 1 || got["results"].([]any)[0].(map[string]any)["text"] != "Grüezi 世" {
			t.Fatal("hints changed extraction or lost the text limit")
		}
		if got["statuses"].([]any)[1].(map[string]any)["status"] != "error" {
			t.Fatal("hints hid a failed extraction")
		}
	}
}

func TestInvalidSupportedOptionsStillFailWithUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"query":"test","future":true,"numResults":"many"}`,
		`{"query":"test","future":true,"contents":[]}`,
		`{"query":"test","future":true,"contents":{"text":42}}`,
		`{"query":"test","future":true,"contents":{"text":{"maxCharacters":"many","future":true}}}`,
		`{"query":"test","future":true,"contents":{"highlights":{"maxCharacters":0,"future":true}}}`,
		`{"query":"test","future":`,
		`{"query":"test","future":true} {}`,
		`[]`,
	} {
		s, providers := webtest.NewServer(t, false)
		webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", body, nil), 400)
		if providers["tavily"].Searches != 0 || providers["tavily"].Scrapes != 0 {
			t.Fatal("invalid supported option reached the provider")
		}
	}
}
