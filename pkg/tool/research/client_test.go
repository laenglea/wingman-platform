package research

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/adrianliechti/wingman/pkg/tool"
)

type fakeResearcher struct {
	instructions string
	result       *researcher.Result
	err          error
}

func (f *fakeResearcher) Research(_ context.Context, instructions string, _ *researcher.ResearchOptions) iter.Seq2[*researcher.Result, error] {
	return func(yield func(*researcher.Result, error) bool) {
		f.instructions = instructions
		yield(f.result, f.err)
	}
}

func TestExecute_ResearchReportAndFailures(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("accepted missing provider")
	}
	f := &fakeResearcher{result: &researcher.Result{Content: "Answer [source](https://example.com)."}}
	c, _ := New(f)
	got, err := c.Execute(t.Context(), ToolName, map[string]any{"instructions": "  Compare sources\n"})
	if err != nil || got != f.result.Content || f.instructions != "Compare sources" {
		t.Fatalf("got %v, instructions %q, err %v", got, f.instructions, err)
	}
	if rendered, _ := tool.RenderResult(c, ToolName, got); rendered.Parts[0].Text != got {
		t.Fatal("report was JSON quoted")
	}
	for _, test := range []struct {
		name   string
		params map[string]any
	}{
		{"wrong", map[string]any{"instructions": "q"}}, {ToolName, nil},
		{ToolName, map[string]any{"instructions": " "}}, {ToolName, map[string]any{"instructions": 42}},
	} {
		f.instructions = ""
		if _, err := c.Execute(t.Context(), test.name, test.params); err == nil || f.instructions != "" {
			t.Fatalf("invalid request reached provider: %+v, err %v", test, err)
		}
	}
	f.result = nil
	if _, err := c.Execute(t.Context(), ToolName, map[string]any{"instructions": "q"}); err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("nil response error: %v", err)
	}
	f.err = errors.New("provider unavailable")
	if _, err := c.Execute(t.Context(), ToolName, map[string]any{"instructions": "q"}); !errors.Is(err, f.err) {
		t.Fatalf("lost provider error: %v", err)
	}
}
