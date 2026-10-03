package exa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/searcher"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSearchRetrievalOnlyAndOptionsUnchanged(t *testing.T) {
	limit := 3
	options := &searcher.SearchOptions{Limit: &limit, Include: []string{"example.com"}, Exclude: []string{"excluded.example"}}
	want := *options
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.exa.ai/search" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["type"] != "fast" || body["query"] != "launch date" || body["numResults"] != float64(3) || body["category"] != "news" || body["userLocation"] != "CH" {
			t.Fatalf("wrong request: %#v", body)
		}
		if !reflect.DeepEqual(body["contents"], map[string]any{"text": true}) {
			t.Fatalf("only plain text retrieval is allowed: %#v", body["contents"])
		}
		for _, name := range []string{"outputSchema", "summary", "highlights", "systemPrompt", "additionalQueries"} {
			if _, ok := body[name]; ok {
				t.Fatalf("unexpected synthesis field %s", name)
			}
		}
		if r.Header.Get("x-api-key") != "test-token" {
			t.Fatal("missing authentication")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"results":[{"url":"https://example.com","title":"Report","text":"Evidence","publishedDate":"2031-05-14T09:00:00Z"}]}`))}, nil
	})}
	c, err := New("test-token", WithClient(httpClient), WithCategory("news"), WithLocation("CH"))
	if err != nil {
		t.Fatal(err)
	}
	results, err := c.Search(context.Background(), " launch date ", options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*options, want) {
		t.Fatalf("mutated caller options: %#v", options)
	}
	if len(results) != 1 || results[0].Content != "Evidence" || results[0].Timestamp == nil || results[0].Metadata["published"] != "2031-05-14T09:00:00Z" {
		t.Fatalf("missing evidence or publication date: %#v", results)
	}
}

func TestSearchRejectsSynthesisModesAndInvalidInput(t *testing.T) {
	for _, mode := range []string{"deep", "deep-lite", "deep-reasoning", "unknown"} {
		if _, err := New("test", WithMode(mode)); err == nil {
			t.Errorf("accepted mode %s", mode)
		}
	}
	c, err := New("test", WithClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid input must not make a request")
		return nil, nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(context.Background(), "  ", nil); err == nil {
		t.Error("accepted empty query")
	}
	for _, limit := range []int{0, -1, 101} {
		if _, err := c.Search(context.Background(), "query", &searcher.SearchOptions{Limit: &limit}); err == nil {
			t.Errorf("accepted limit %d", limit)
		}
	}
}

func TestSearchExcerptFindsLateEvidenceWithoutSynthesizing(t *testing.T) {
	page := strings.Repeat("Routine operations and background. ", 800) + "\nLaunch date: 14 May 2031.\n" + strings.Repeat("Appendix. ", 1000)
	got := searchExcerpt(page, "launch date")
	if !strings.Contains(got, "Launch date: 14 May 2031.") {
		t.Fatal("lost late evidence")
	}
	if len([]rune(got)) > 1500 {
		t.Fatal("excerpt exceeds the frontend snippet budget")
	}
	_, body, _ := strings.Cut(got, "\n")
	if !strings.Contains(page, body) {
		t.Fatal("excerpt was not verbatim")
	}
}

func TestSearchExcerptUnicodeAndMissingMatches(t *testing.T) {
	page := strings.Repeat("背景。", 1500) + "公開日：2031年5月14日。" + strings.Repeat("付録。", 800)
	if got := searchExcerpt(page, "公開日"); !strings.Contains(got, "公開日：2031年5月14日。") {
		t.Fatal("lost Unicode evidence")
	}
	if got := searchExcerpt("  Short text.\n", "other"); got != "  Short text.\n" {
		t.Fatal("changed short text")
	}
	if got := searchExcerpt(strings.Repeat("a", 2000), "missing"); !strings.HasSuffix(got, strings.Repeat("a", excerptCharacters)) {
		t.Fatal("missing prefix fallback")
	}
}
