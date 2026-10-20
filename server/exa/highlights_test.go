package exa_test

import (
	"testing"

	"github.com/adrianliechti/wingman/test/webtest"
)

func TestHighlightsAcrossProviders(t *testing.T) {
	for _, backend := range []string{"tavily", "exa", "plain"} {
		for _, selector := range []string{"true", "{}", `{"dynamic":true}`, `{"dynamic":true,"verbosity":"high"}`, `{"maxCharacters":4}`} {
			s, providers := webtest.NewServer(t, false)
			got := webtest.Response(t, webtest.Send(t, s, "/v1/exa/search", `{"model":"`+backend+`","query":"test","contents":{"highlights":`+selector+`}}`, nil), 200)
			hit := got["results"].([]any)[0].(map[string]any)
			want := "Evidence"
			if selector == `{"maxCharacters":4}` {
				want = "Evid"
			}
			if hit["highlights"].([]any)[0] != want || providers[backend].Scrapes != 0 || (providers[backend].Options.Text != nil) {
				t.Fatal("highlights fetched full text or lost search evidence")
			}
		}
	}
}
