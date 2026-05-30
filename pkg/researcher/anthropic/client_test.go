package anthropic

import (
	"strings"
	"testing"
)

func TestAppendSources(t *testing.T) {
	if got := appendSources("answer", nil); got != "answer" {
		t.Errorf("no sources: got %q, want %q", got, "answer")
	}

	sources := []source{
		{title: "Example", url: "https://example.com"},
		{title: "", url: "https://example.org"},
	}

	got := appendSources("answer", sources)

	if !strings.HasPrefix(got, "answer\n\nSources:") {
		t.Errorf("expected content followed by sources, got %q", got)
	}

	if !strings.Contains(got, "Example: https://example.com") {
		t.Errorf("expected titled source line, got %q", got)
	}

	if !strings.Contains(got, "- https://example.org") {
		t.Errorf("expected untitled source line, got %q", got)
	}
}
