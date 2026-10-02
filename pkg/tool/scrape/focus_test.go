package scrape

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adrianliechti/wingman/pkg/scraper"
)

func TestFocusedText_PreservesLateEvidenceAndOffsets(t *testing.T) {
	text := strings.Repeat("Routine background. ", 2000) + "\n# Launch budget\nLaunch: 14 May 2031. Budget: 83 million credits.\n" + strings.Repeat("界", 3000)
	got := focusedText(text, "launch budget", 2000)
	for _, want := range []string{"Selected excerpts", "Characters ", "Launch: 14 May 2031. Budget: 83 million credits."} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if utf8.RuneCountInString(got) > 2000 || !utf8.ValidString(got) {
		t.Fatal("invalid output budget or unicode")
	}
	for range 5 {
		if focusedText(text, "launch budget", 2000) != got {
			t.Fatal("nondeterministic ranking")
		}
	}
}

func TestFocusedText_CoversSeparateClaimsAndNoMatchFallback(t *testing.T) {
	text := "Launch: 14 May 2031.\n" + strings.Repeat("Ordinary background. ", 500) + "Budget: 83 million credits."
	got := focusedText(text, "launch budget", 4000)
	if !strings.Contains(got, "14 May 2031") || !strings.Contains(got, "83 million credits") {
		t.Fatal("lost separate claim")
	}
	if !strings.Contains(focusedText(text, "unrelatedxyz", 2000), "No keyword matches") {
		t.Fatal("missing fallback notice")
	}
}

func TestExecute_SequentialOffsetOverridesFocus(t *testing.T) {
	c, _ := New(&fakeScraper{doc: &scraper.Document{Text: "abcdef"}}, WithMaxChars(3))
	got, err := c.Execute(context.Background(), ToolName, map[string]any{"url": "https://example.com", "query": "abc", "start_index": float64(3)})
	if err != nil || !strings.HasSuffix(got.(string), "def") {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, params := range []map[string]any{{"start_index": -1.0}, {"start_index": 1.5}, {"max_chars": 0.0}} {
		params["url"] = "https://example.com"
		if _, err := c.Execute(context.Background(), ToolName, params); err == nil {
			t.Fatalf("accepted invalid args %v", params)
		}
	}
}

func TestExecute_ZeroOrNullOffsetAllowsFocusedReading(t *testing.T) {
	text := strings.Repeat("background ", 1000) + "Launch: 14 May 2031."
	c, _ := New(&fakeScraper{doc: &scraper.Document{Text: text}}, WithMaxChars(2000))
	for _, offset := range []any{float64(0), nil} {
		got, err := c.Execute(context.Background(), ToolName, map[string]any{"url": "https://example.com", "query": "launch", "start_index": offset})
		if err != nil || !strings.Contains(got.(string), "14 May 2031") {
			t.Fatalf("got=%v err=%v", got, err)
		}
	}
}

func TestFocusedText_SmallBudgetPreservesLateEvidence(t *testing.T) {
	text := strings.Repeat("background ", 10000) + "Launch: 14 May 2031."
	got := focusedText(text, "Launch", 256)
	if !strings.Contains(got, "14 May 2031") || utf8.RuneCountInString(got) > 256 {
		t.Fatalf("invalid excerpt: %s", got)
	}
}

func TestExecute_RejectsInvalidFocusBeforeFetching(t *testing.T) {
	s := &fakeScraper{}
	c, _ := New(s)
	for _, query := range []any{42, strings.Repeat("界", 501)} {
		_, err := c.Execute(context.Background(), ToolName, map[string]any{"url": "https://example.com", "query": query})
		if err == nil || s.url != "" {
			t.Fatalf("err=%v fetched=%q", err, s.url)
		}
	}
}
