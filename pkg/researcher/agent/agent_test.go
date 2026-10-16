package agent

import (
	"context"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/adrianliechti/wingman/pkg/searcher"
)

type fakeSearcher struct{}

func (f *fakeSearcher) Search(ctx context.Context, q string, o *searcher.SearchOptions) ([]searcher.Result, error) {
	return []searcher.Result{{Source: "https://example.com", Title: "Example", Excerpt: "body"}}, nil
}

func (f *fakeSearcher) SearchCapabilities() searcher.Capabilities {
	return searcher.Capabilities{DateFilters: true}
}

type completerCall struct {
	messages []provider.Message
	options  *provider.CompleteOptions
}

type fakeCompleter struct {
	calls  []completerCall
	script []provider.Completion
}

func (f *fakeCompleter) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	completion := f.script[len(f.calls)]
	f.calls = append(f.calls, completerCall{messages: slices.Clone(messages), options: options})

	return func(yield func(*provider.Completion, error) bool) {
		yield(&completion, nil)
	}
}

func assistantToolCalls(calls ...provider.ToolCall) provider.Message {
	m := provider.Message{Role: provider.MessageRoleAssistant}
	for _, tc := range calls {
		m.Content = append(m.Content, provider.ToolCallContent(tc))
	}
	return m
}

func TestResearchProgress(t *testing.T) {
	first := assistantToolCalls(provider.ToolCall{ID: "1", Name: "web_search", Arguments: `{"query":"evidence"}`})
	final := provider.AssistantMessage("cited report")
	c, err := New(&fakeCompleter{script: []provider.Completion{{Message: &first}, {Message: &final}}}, &fakeSearcher{})
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	var report *researcher.Result
	for result, err := range c.Research(t.Context(), "question", nil) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Progress != "" {
			messages = append(messages, result.Progress)
		} else {
			report = result
		}
	}
	if report == nil || report.Content != "cited report" {
		t.Fatalf("result = %+v", report)
	}
	want := []string{"Planning research", "Gathering evidence: 1 search requests, 0 source reads; evidence", "Processed 1 research tool calls", "Reviewing evidence and planning next steps", "Drafting research response"}
	if !slices.Equal(messages, want) {
		t.Fatalf("progress = %v, want %v", messages, want)
	}
}

func TestResearchStopsWhenConsumerBreaks(t *testing.T) {
	answer := provider.AssistantMessage("answer")
	completer := &fakeCompleter{script: []provider.Completion{{Message: &answer}}}
	c, err := New(completer, &fakeSearcher{})
	if err != nil {
		t.Fatal(err)
	}
	stream := c.Research(t.Context(), "question", nil)
	if len(completer.calls) != 0 {
		t.Fatal("research ran before iteration")
	}
	for result, err := range stream {
		if err != nil || result.Progress != "Planning research" {
			t.Fatalf("first result = %+v, error = %v", result, err)
		}
		break
	}
	if len(completer.calls) != 0 {
		t.Fatal("model called after consumer stopped at planning")
	}
}

func TestResearch_ContinuesBeyondAdvisoryTarget(t *testing.T) {
	first := assistantToolCalls(
		provider.ToolCall{ID: "1", Name: "web_search", Arguments: `{"query":"a"}`},
		provider.ToolCall{ID: "2", Name: "web_search", Arguments: `{"query":"b"}`},
		provider.ToolCall{ID: "3", Name: "web_search", Arguments: `{"query":"c"}`},
	)
	followup := assistantToolCalls(provider.ToolCall{ID: "4", Name: "web_search", Arguments: `{"query":"remaining gap"}`})
	final := provider.AssistantMessage("final answer")
	completer := &fakeCompleter{script: []provider.Completion{{Message: &first}, {Message: &followup}, {Message: &final}}}
	c, err := New(completer, &fakeSearcher{}, WithToolCallTarget(2))
	if err != nil {
		t.Fatal(err)
	}
	result, err := researcher.Collect(c.Research(t.Context(), "question", nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "final answer" || len(completer.calls) != 3 {
		t.Fatalf("result=%+v completion calls=%d", result, len(completer.calls))
	}
	ids := map[string]string{}
	for _, message := range completer.calls[2].messages {
		if r, ok := message.ToolResult(); ok {
			ids[r.ID] = r.Parts[0].Text
		}
	}
	for _, id := range []string{"1", "2", "3", "4"} {
		if !strings.Contains(ids[id], "body") || strings.Contains(ids[id], "Error:") {
			t.Fatalf("tool %s did not execute: %q", id, ids[id])
		}
	}
	if !strings.Contains(ids["3"], "Prefer answering from the evidence gathered") {
		t.Fatal("missing efficiency hint")
	}
	for _, call := range completer.calls {
		if len(call.options.Tools) == 0 {
			t.Fatal("tools removed after the advisory target")
		}
	}
}

func TestResearch_StopsWithoutToolCalls(t *testing.T) {
	answer := provider.AssistantMessage("direct answer")

	completer := &fakeCompleter{
		script: []provider.Completion{{Message: &answer}},
	}

	c, err := New(completer, &fakeSearcher{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := researcher.Collect(c.Research(context.Background(), "question", nil))
	if err != nil {
		t.Fatalf("Research: %v", err)
	}
	if result.Content != "direct answer" {
		t.Errorf("content = %q", result.Content)
	}
	if len(completer.calls) != 1 {
		t.Errorf("completer calls = %d, want 1", len(completer.calls))
	}
}

func TestResearch_RejectsTruncatedOrEmptyAnswer(t *testing.T) {
	partial := provider.AssistantMessage("partial rep")

	for name, completion := range map[string]provider.Completion{
		"truncated": {Status: provider.CompletionStatusIncomplete, StopReason: provider.StopReasonMaxTokens, Message: &partial},
		"failed":    {Status: provider.CompletionStatusFailed},
		"empty":     {},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := New(&fakeCompleter{script: []provider.Completion{completion}}, &fakeSearcher{})
			if err != nil {
				t.Fatal(err)
			}
			if result, err := researcher.Collect(c.Research(context.Background(), "question", nil)); err == nil {
				t.Fatalf("accepted %s answer: %+v", name, result)
			}
		})
	}
}
