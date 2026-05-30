package anthropic

import (
	"context"
	"strings"

	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/anthropics/anthropic-sdk-go"
)

var _ researcher.Provider = &Client{}

// webToolsBeta enables dynamic filtering: Claude post-processes web search
// results with the code execution tool before they enter the context window.
const webToolsBeta anthropic.AnthropicBeta = "code-execution-web-tools-2026-02-09"

type Client struct {
	*Config
	messages anthropic.BetaMessageService
}

func New(token string, options ...Option) (*Client, error) {
	cfg := &Config{
		token: token,
	}

	for _, option := range options {
		option(cfg)
	}

	return &Client{
		Config:   cfg,
		messages: anthropic.NewBetaMessageService(cfg.Options()...),
	}, nil
}

func (c *Client) Research(ctx context.Context, instructions string, options *researcher.ResearchOptions) (*researcher.Result, error) {
	if options == nil {
		options = new(researcher.ResearchOptions)
	}

	model := c.model

	if model == "" {
		model = "claude-sonnet-4-6"
	}

	body := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 8192,

		Betas: []anthropic.AnthropicBeta{webToolsBeta},

		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(instructions)),
		},

		System: []anthropic.BetaTextBlockParam{
			{Text: "Use web search to gather current, source-backed information. Include citations in the final answer."},
		},

		Tools: []anthropic.BetaToolUnionParam{
			{
				OfWebSearchTool20260209: &anthropic.BetaWebSearchTool20260209Param{
					MaxUses: anthropic.Int(5),
				},
			},
			{
				OfCodeExecutionTool20260120: &anthropic.BetaCodeExecutionTool20260120Param{},
			},
		},
	}

	message, err := c.messages.New(ctx, body)

	if err != nil {
		return nil, err
	}

	var content strings.Builder
	var sources []source
	seen := map[string]struct{}{}

	for _, c := range message.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)

			for _, citation := range c.Citations {
				if citation.Type != "web_search_result_location" {
					continue
				}

				result := citation.AsWebSearchResultLocation()

				if result.URL == "" {
					continue
				}

				if _, ok := seen[result.URL]; ok {
					continue
				}

				seen[result.URL] = struct{}{}
				sources = append(sources, source{
					title: result.Title,
					url:   result.URL,
				})
			}
		}
	}

	result := &researcher.Result{
		Content: appendSources(strings.TrimSpace(content.String()), sources),
	}

	return result, nil
}

type source struct {
	title string
	url   string
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
