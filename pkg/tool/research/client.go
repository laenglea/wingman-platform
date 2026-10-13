package research

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/adrianliechti/wingman/pkg/researcher"
	"github.com/adrianliechti/wingman/pkg/tool"
)

const ToolName = "web_research"

var (
	_ tool.Streamer = (*Client)(nil)
	_ tool.Provider = (*Client)(nil)
)

type Client struct {
	provider researcher.Provider
}

func New(provider researcher.Provider, options ...Option) (*Client, error) {
	if provider == nil {
		return nil, errors.New("research: missing researcher provider")
	}

	c := &Client{
		provider: provider,
	}

	for _, option := range options {
		option(c)
	}

	return c, nil
}

func (c *Client) Tools(ctx context.Context) ([]tool.Tool, error) {
	return []tool.Tool{
		{
			Name:        ToolName,
			Description: "Delegate a complete question that needs multiple source reads, source comparison, cross-source verification, or dependent search steps. Call this first for those tasks, including quick comparisons of supplied URLs; the researcher performs the searches and reads and returns a cited report. Include all known URLs and constraints in instructions. Use web_search for an excerpt-level lookup or independent lookup batch; use web_fetch when one supplied page alone is sufficient.",

			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"instructions": map[string]any{
						"type":        "string",
						"description": "A clear, self-contained description of what to research. Include the question, any constraints (timeframe, sources to prefer), and the shape of the answer you want.",
					},
				},
				"required": []string{"instructions"},
			},
		},
	}, nil
}

func (c *Client) Execute(ctx context.Context, name string, parameters map[string]any) (any, error) {
	return tool.Collect(c.ExecuteStream(ctx, name, parameters), nil)
}

func (c *Client) ExecuteStream(ctx context.Context, name string, parameters map[string]any) iter.Seq2[*tool.Execution, error] {
	return func(yield func(*tool.Execution, error) bool) {
		if name != ToolName {
			yield(nil, tool.ErrInvalidTool)
			return
		}
		instructions, err := tool.StringParameter(parameters, "instructions")
		if err != nil {
			yield(nil, fmt.Errorf("research: %w", err))
			return
		}
		instructions = strings.TrimSpace(instructions)
		if instructions == "" {
			yield(nil, errors.New("research: missing instructions parameter"))
			return
		}
		var final *researcher.Result
		for result, err := range c.provider.Research(ctx, instructions, &researcher.ResearchOptions{}) {
			if err != nil {
				if !yield(&tool.Execution{Progress: "Research failed"}, nil) {
					return
				}
				yield(nil, err)
				return
			}
			if result == nil {
				continue
			}
			if result.Progress != "" {
				if !yield(&tool.Execution{Progress: result.Progress}, nil) {
					return
				}
			} else {
				final = result
			}
		}
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		if final == nil {
			yield(nil, errors.New("research: empty response"))
			return
		}
		if !yield(&tool.Execution{Progress: "Research complete"}, nil) {
			return
		}
		yield(&tool.Execution{Result: final.Content}, nil)
	}
}
