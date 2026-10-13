package openai

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/researcher"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

var _ researcher.Provider = &Client{}

type Client struct {
	*Config
	responses responses.ResponseService
}

func New(token string, options ...Option) (*Client, error) {
	cfg := &Config{
		token: token,
	}

	for _, option := range options {
		option(cfg)
	}

	return &Client{
		Config:    cfg,
		responses: responses.NewResponseService(cfg.Options()...),
	}, nil
}

func (c *Client) Research(ctx context.Context, instructions string, options *researcher.ResearchOptions) iter.Seq2[*researcher.Result, error] {
	return func(yield func(*researcher.Result, error) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		if !yield(&researcher.Result{Progress: "Starting research"}, nil) {
			return
		}
		model := c.model

		if model == "" {
			model = "gpt-5.5"
		}

		tool := responses.ToolUnionParam{
			OfWebSearch: &responses.WebSearchToolParam{
				Type: responses.WebSearchToolTypeWebSearch,
			},
		}

		if strings.Contains(model, "deep-research") {
			tool = responses.ToolUnionParam{
				OfWebSearchPreview: &responses.WebSearchPreviewToolParam{
					Type: responses.WebSearchPreviewToolTypeWebSearchPreview,
				},
			}
		}

		if tool.OfWebSearch != nil {
			tool.OfWebSearch.SearchContextSize = responses.WebSearchToolSearchContextSizeHigh
		}

		if tool.OfWebSearchPreview != nil {
			tool.OfWebSearchPreview.SearchContextSize = responses.WebSearchPreviewToolSearchContextSizeHigh
		}

		body := responses.ResponseNewParams{
			Model: responses.ResponsesModel(model),

			Instructions: openai.String("Research the user's question using web sources. Honor the requested timeframes, document versions, and source restrictions. Prefer primary sources and cite their URLs. If the evidence is insufficient, say so. Return a self-contained report that follows the requested length and format."),

			Input: responses.ResponseNewParamsInputUnion{
				OfString: openai.String(instructions),
			},

			Include: []responses.ResponseIncludable{
				responses.ResponseIncludableWebSearchCallActionSources,
			},

			Tools: []responses.ToolUnionParam{tool},
		}

		stream := c.responses.NewStreaming(ctx, body)
		defer stream.Close()
		lastProgress := ""
		for stream.Next() {
			event := stream.Current()
			progress := ""
			switch event.Type {
			case "response.queued":
				progress = "Research queued"
			case "response.in_progress":
				progress = "Planning research"
			case "response.web_search_call.in_progress":
				progress = "Starting web search"
			case "response.web_search_call.searching":
				progress = "Searching the web"
			case "response.web_search_call.completed":
				progress = "Web search complete; reviewing sources"
			case "response.output_item.added", "response.output_item.done":
				switch event.Item.Type {
				case "reasoning":
					progress = "Analyzing research evidence"
				case "web_search_call":
					action := event.Item.Action
					switch action.Type {
					case "search":
						queries := action.Queries
						if len(queries) == 0 && action.Query != "" {
							queries = []string{action.Query}
						}
						if len(queries) > 0 {
							progress = "Search queries: " + strings.Join(queries, "; ")
						}
					case "open_page":
						progress = "Reading source: " + action.URL
					case "find_in_page":
						progress = "Finding source evidence: " + action.URL
					}
				}
			case "response.output_text.delta":
				progress = "Writing research report"
			case "response.completed":
				yield(&researcher.Result{Content: outputText(&event.Response)}, nil)
				return
			case "response.failed":
				yield(nil, fmt.Errorf("openai research failed: %s (%s)", event.Response.Error.Message, event.Response.Error.Code))
				return
			case "response.incomplete":
				yield(nil, fmt.Errorf("openai research incomplete: %s", event.Response.IncompleteDetails.Reason))
				return
			case "error":
				yield(nil, fmt.Errorf("openai research: %s", event.Message))
				return
			}
			if progress != "" && progress != lastProgress {
				lastProgress = progress
				if !yield(&researcher.Result{Progress: progress}, nil) {
					return
				}
			}
		}
		if err := stream.Err(); err != nil {
			yield(nil, err)
			return
		}
		yield(nil, errors.New("openai research: stream ended before response.completed"))
	}
}

type source struct {
	title string
	url   string
}

func outputText(response *responses.Response) string {
	var messages []responses.ResponseOutputItemUnion
	var phases []provider.MessagePhase
	for _, item := range response.Output {
		if item.Type == "message" {
			messages = append(messages, item)
			phases = append(phases, provider.MessagePhase(item.Phase))
		}
	}
	i := provider.FinalMessageIndex(phases)
	if i < 0 {
		return ""
	}

	var content strings.Builder

	var sources []source
	seen := map[string]struct{}{}

	for _, c := range messages[i].Content {
		if c.Type == "refusal" {
			return ""
		}
		if c.Type != "output_text" {
			continue
		}

		content.WriteString(c.Text)

		for _, a := range c.Annotations {
			if a.Type != "url_citation" || a.URL == "" {
				continue
			}

			if _, ok := seen[a.URL]; ok {
				continue
			}

			seen[a.URL] = struct{}{}
			sources = append(sources, source{
				title: a.Title,
				url:   a.URL,
			})
		}
	}

	return appendSources(strings.TrimSpace(content.String()), sources)
}

func appendSources(content string, sources []source) string {
	if len(sources) == 0 {
		return content
	}

	var b strings.Builder

	if content != "" {
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	b.WriteString("Sources:")

	for _, source := range sources {
		b.WriteString("\n- ")

		if source.title != "" {
			b.WriteString(source.title)
			b.WriteString(": ")
		}

		b.WriteString(source.url)
	}

	return b.String()
}
