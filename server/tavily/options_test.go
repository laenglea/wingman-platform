package tavily_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/webtest"
)

func TestTuningHintsAcrossProviders(t *testing.T) {
	for _, backend := range []string{"exa", "tavily", "plain"} {
		for _, depth := range []string{"basic", "advanced", "fast", "ultra-fast", "future-depth"} {
			t.Run(backend+"/"+depth, func(t *testing.T) {
				s, providers := webtest.NewServer(t, false)
				body := `{"model":"` + backend + `","query":"test","max_results":2,
					"include_domains":["example.com"],"search_depth":"` + depth + `",
					"chunks_per_source":2,"language":"german","include_usage":true,"include_favicon":true,
					"include_answer":true,"include_images":true,"auto_parameters":true,"future_option":{"enabled":true}}`
				got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/search", body, nil), 200)
				hit := got["results"].([]any)[0].(map[string]any)
				p := providers[backend]
				if p.Searches != 1 || p.Scrapes != 0 || p.Query != "test" || *p.Options.Limit != 2 || p.Options.Include[0] != "example.com" || hit["content"] != "Evidence" {
					t.Fatalf("hints changed retrieval: %+v", p.Options)
				}
				if got["usage"] != nil || got["answer"] != nil || len(got["images"].([]any)) != 0 || hit["favicon"] != nil || hit["raw_content"] != nil {
					t.Fatal("hints invented metadata or requested full text")
				}
			})
		}
	}
}

func TestExtractAcceptsDepthAndMetadataHints(t *testing.T) {
	for _, backend := range []string{"exa", "tavily", "plain"} {
		for _, depth := range []string{"basic", "advanced", "future-depth"} {
			s, providers := webtest.NewServer(t, false)
			body := `{"model":"` + backend + `","urls":["https://example.com","https://example.com/missing"],
				"extract_depth":"` + depth + `","chunks_per_source":5,"include_usage":true,"include_favicon":true,
				"query":"select passages","format":"text","include_images":true,"future_option":{"enabled":true}}`
			got := webtest.Response(t, webtest.Send(t, s, "/v1/tavily/extract", body, nil), 200)
			if providers[backend].Scrapes != 2 || len(got["results"].([]any)) != 1 || len(got["failed_results"].([]any)) != 1 {
				t.Fatal("hints changed batch extraction or hid failures")
			}
			if got["results"].([]any)[0].(map[string]any)["raw_content"] != "Grüezi 世界 — full page" || got["usage"] != nil {
				t.Fatal("hints changed extracted text or invented usage")
			}
		}
	}
}

func TestInvalidSupportedOptionsStillFailWithUnknownFields(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"search", `{"query":"test","future":true,"max_results":"many"}`},
		{"search", `{"query":"test","future":true,"include_raw_content":{}}`},
		{"extract", `{"urls":[],"future":true}`},
		{"extract", `{"urls":"https://example.com","future":true,"timeout":0}`},
	} {
		s, providers := webtest.NewServer(t, false)
		webtest.Response(t, webtest.Send(t, s, "/v1/tavily/"+tc.path, tc.body, nil), 400)
		if providers["tavily"].Searches != 0 || providers["tavily"].Scrapes != 0 {
			t.Fatal("invalid supported option reached the provider")
		}
	}
}
