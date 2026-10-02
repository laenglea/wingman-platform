package agent

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/adrianliechti/wingman/pkg/scraper"
	"github.com/adrianliechti/wingman/pkg/searcher"
	"github.com/adrianliechti/wingman/pkg/template"
	"github.com/adrianliechti/wingman/pkg/tool"
	"github.com/adrianliechti/wingman/pkg/tool/scrape"
	"github.com/adrianliechti/wingman/pkg/tool/search"
)

var _ researcher.Provider = &Client{}

//go:embed agent.md
var systemPromptSource string

const (
	defaultMaxToolCalls       = 20
	defaultMaxFetchChars      = 6000
	defaultMaxTotalFetchChars = 80 * 1024
	defaultSummarizeMinChars  = 4 * 1024

	toolWebSearch = "web_search"
	toolWebFetch  = "web_fetch"
)

const finalizePrompt = "The tool-call budget is exhausted. Do not request more tools. Write the final answer now using only the evidence already gathered, with inline citations to the sources you retrieved. If the evidence is incomplete, state exactly what is missing."

type Client struct {
	completer provider.Completer

	searcher   searcher.Provider
	scraper    scraper.Provider
	summarizer provider.Completer

	effort    provider.Effort
	verbosity provider.Verbosity

	maxToolCalls       int
	maxFetchChars      int
	maxTotalFetchChars int
	summarizeMinChars  int

	prompt *template.Template
}

func New(completer provider.Completer, searcher searcher.Provider, options ...Option) (*Client, error) {
	if completer == nil {
		return nil, errors.New("research: missing completer provider")
	}
	if searcher == nil {
		return nil, errors.New("research: missing searcher provider")
	}
	prompt, err := template.NewTemplate(systemPromptSource)
	if err != nil {
		return nil, err
	}

	c := &Client{
		completer: completer,
		searcher:  searcher,

		maxToolCalls:       defaultMaxToolCalls,
		maxFetchChars:      defaultMaxFetchChars,
		maxTotalFetchChars: defaultMaxTotalFetchChars,
		summarizeMinChars:  defaultSummarizeMinChars,

		prompt: prompt,
	}

	for _, option := range options {
		option(c)
	}

	return c, nil
}

func (c *Client) Research(ctx context.Context, instructions string, options *researcher.ResearchOptions) (*researcher.Result, error) {
	prompt, err := c.prompt.Execute(map[string]any{
		"HasScraper":   c.scraper != nil,
		"MaxToolCalls": c.maxToolCalls,
	})
	if err != nil {
		return nil, err
	}

	searchProvider, err := search.New(&cachedSearcher{Provider: c.searcher}, search.WithMaxSnippetChars(1500))
	if err != nil {
		return nil, err
	}

	tools := map[string]tool.Provider{}
	var toolDefs []provider.Tool

	searchTools, err := searchProvider.Tools(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range searchTools {
		tools[t.Name] = searchProvider
		toolDefs = append(toolDefs, t)
	}

	if c.scraper != nil {
		scrapeProvider, err := scrape.New(&cachedScraper{Provider: c.scraper}, scrape.WithMaxChars(c.maxFetchChars))
		if err != nil {
			return nil, err
		}
		scrapeTools, err := scrapeProvider.Tools(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range scrapeTools {
			tools[t.Name] = scrapeProvider
			toolDefs = append(toolDefs, t)
		}
	}

	messages := []provider.Message{
		provider.SystemMessage(prompt),
		provider.UserMessage(instructions),
	}

	completeOptions := &provider.CompleteOptions{
		Tools: toolDefs,
	}
	if c.verbosity != "" {
		completeOptions.OutputOptions = &provider.OutputOptions{Verbosity: c.verbosity}
	}
	if c.effort != "" {
		completeOptions.ReasoningOptions = &provider.ReasoningOptions{Effort: c.effort}
	}

	s := &state{
		instructions: instructions,
		tools:        tools,
		client:       c,
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		exhausted := s.toolCalls >= c.maxToolCalls

		opts := completeOptions
		if exhausted {
			final := *completeOptions
			final.Tools = nil
			opts = &final

			messages = append(messages, provider.UserMessage(finalizePrompt))
		}

		acc := provider.CompletionAccumulator{}
		for completion, err := range c.completer.Complete(ctx, messages, opts) {
			if err != nil {
				return nil, err
			}
			if completion != nil {
				acc.Add(*completion)
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		result := acc.Result()
		if result.Message == nil {
			return &researcher.Result{Content: ""}, nil
		}

		messages = append(messages, *result.Message)

		calls := result.Message.ToolCalls()
		if exhausted || len(calls) == 0 {
			return &researcher.Result{Content: result.Text()}, nil
		}

		remaining := c.maxToolCalls - s.toolCalls

		run := calls
		var skipped []provider.ToolCall
		if len(calls) > remaining {
			run, skipped = calls[:remaining], calls[remaining:]
		}
		s.toolCalls += len(run)

		toolMessages := s.runCalls(ctx, run)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, tc := range skipped {
			toolMessages = append(toolMessages, provider.ToolMessage(tc.ID, "Error: tool-call budget exhausted; this call was not executed."))
		}

		if remaining := c.maxToolCalls - s.toolCalls; remaining > 0 && remaining <= max(2, c.maxToolCalls/5) {
			appendText(&toolMessages[len(toolMessages)-1], fmt.Sprintf("\n\n[%d tool call(s) remaining — close the most important gap, then answer]", remaining))
		}

		messages = append(messages, toolMessages...)
	}
}

type state struct {
	instructions string
	tools        map[string]tool.Provider
	client       *Client

	toolCalls    int
	fetchedChars int
	seenEvidence map[[32]byte]string
}

func (s *state) runCalls(ctx context.Context, calls []provider.ToolCall) []provider.Message {
	results := make([]provider.Message, len(calls))
	// Allocate in call order, then account for actual output after all workers
	// finish. Workers only write their own result; budget state stays sequential.
	budgets := make([]int, len(calls))
	used := make([]int, len(calls))
	remaining := s.client.maxTotalFetchChars - s.fetchedChars
	jobs := make(chan int, len(calls))
	for i, tc := range calls {
		if tc.Name == toolWebFetch {
			budgets[i] = max(0, min(s.client.maxFetchChars+512, remaining))
			remaining -= budgets[i]
		}
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for range min(4, len(calls)) {
		wg.Go(func() {
			for i := range jobs {
				results[i], used[i] = s.runCall(ctx, calls[i], budgets[i])
			}
		})
	}
	wg.Wait()
	for i := range results {
		// The retrieval caches avoid repeated network requests. Keep repeated
		// successful evidence out of subsequent model inputs as well; the first
		// full result remains in the transcript with its original citations.
		result, ok := results[i].ToolResult()
		if ok && len(result.Parts) == 1 {
			text := result.Parts[0].Text
			if compact := s.compactEvidence(calls[i], text); compact != text {
				results[i] = provider.ToolMessage(calls[i].ID, compact)
				if used[i] > 0 {
					used[i] = utf8.RuneCountInString(compact)
				}
			}
		}
		s.fetchedChars += used[i]
	}

	return results
}

func (s *state) compactEvidence(call provider.ToolCall, text string) string {
	if call.ID == "" || len(text) <= 512 || strings.HasPrefix(text, "Error:") {
		return text
	}
	key := sha256.Sum256([]byte(call.Name + "\x00" + text))
	if previous, found := s.seenEvidence[key]; found {
		reference := fmt.Sprintf("Same evidence as tool call %s; reuse its original text and sources. No new evidence.", previous)
		if len(reference) < len(text) && utf8.RuneCountInString(reference) < utf8.RuneCountInString(text) {
			return reference
		}
	} else {
		if s.seenEvidence == nil {
			s.seenEvidence = make(map[[32]byte]string)
		}
		s.seenEvidence[key] = call.ID
	}
	return text
}

func (s *state) runCall(ctx context.Context, tc provider.ToolCall, fetchBudget int) (provider.Message, int) {
	used := 0
	if tc.Name == toolWebFetch && fetchBudget <= 0 {
		return provider.ToolMessage(tc.ID, "Error: total fetch budget exhausted; use existing evidence and state any gaps"), 0
	}
	if err := ctx.Err(); err != nil {
		return provider.ToolMessage(tc.ID, "Error: "+err.Error()), 0
	}
	p, found := s.tools[tc.Name]
	if !found {
		return provider.ToolMessage(tc.ID, "Error: unknown tool"), 0
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &params); err != nil {
		return provider.ToolMessage(tc.ID, "Error: invalid arguments"), 0
	}
	if params == nil {
		return provider.ToolMessage(tc.ID, "Error: invalid arguments"), 0
	}
	if tc.Name == toolWebFetch {
		// Leave space for source/excerpt labels. The final clamp also bounds
		// long URLs, notices and optional summarizer output.
		limit := max(1, min(s.client.maxFetchChars, fetchBudget-512))
		if n, ok := params["max_chars"].(float64); params["max_chars"] == nil || (ok && n > float64(limit)) {
			params["max_chars"] = float64(limit)
		}
	}

	value, err := p.Execute(ctx, tc.Name, params)
	if err != nil {
		return provider.ToolMessage(tc.ID, "Error: "+err.Error()), 0
	}

	text := renderResult(p, tc.Name, value)

	if tc.Name == toolWebFetch {
		if s.client.summarizer != nil && utf8.RuneCountInString(text) >= s.client.summarizeMinChars {
			if summary := s.client.summarize(ctx, s.instructions, text); summary != "" {
				text = summary
			}
		}
		text = limitFetchText(text, fetchBudget)
		used = utf8.RuneCountInString(text)
	}

	return provider.ToolMessage(tc.ID, text), used
}

func limitFetchText(text string, budget int) string {
	chars := []rune(text)
	if len(chars) <= budget {
		return text
	}
	notice := []rune("\n[Fetch budget truncated; omitted text is not evidence.]")
	if budget <= len(notice) {
		return string(notice[:budget])
	}
	return string(chars[:budget-len(notice)]) + string(notice)
}

func (c *Client) summarize(ctx context.Context, instructions, page string) string {
	if c.summarizer == nil {
		return ""
	}

	messages := []provider.Message{
		provider.SystemMessage(`You extract evidence from a fetched web page for a research task. Keep the "Source:" line at the top, then list every fact relevant to the question — preserve exact figures, dates, proper names, and short verbatim quotes where the wording matters. Keep any trailing truncation notice verbatim. Drop navigation, ads, boilerplate, and unrelated sections. If nothing on the page is relevant, reply exactly: Not relevant: <one-line reason>.`),
		provider.UserMessage(fmt.Sprintf("Research question:\n%s\n\nPage:\n%s", instructions, page)),
	}

	acc := provider.CompletionAccumulator{}
	for completion, err := range c.summarizer.Complete(ctx, messages, nil) {
		if err != nil {
			return ""
		}
		if completion != nil {
			acc.Add(*completion)
		}
	}
	return acc.Result().Text()
}

func appendText(m *provider.Message, text string) {
	for i := range m.Content {
		if r := m.Content[i].ToolResult; r != nil && len(r.Parts) > 0 {
			r.Parts[len(r.Parts)-1].Text += text
			return
		}
	}
}

func renderResult(p tool.Provider, name string, value any) string {
	if r, ok := p.(tool.Resulter); ok {
		res := r.Result(name, value)
		if len(res.Parts) > 0 && res.Parts[0].Text != "" {
			return res.Parts[0].Text
		}
	}
	if s, ok := value.(string); ok {
		return s
	}
	data, _ := json.Marshal(value)
	return string(data)
}
