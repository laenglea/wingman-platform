package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"iter"
	"os/exec"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

var _ provider.Completer = (*Completer)(nil)

type Completer struct {
	*Config
}

func NewCompleter(model string, options ...Option) (*Completer, error) {
	cfg := &Config{
		model:   model,
		command: "claude",
	}

	for _, option := range options {
		option(cfg)
	}

	if cfg.command == "" {
		cfg.command = "claude"
	}

	return &Completer{Config: cfg}, nil
}

func (c *Completer) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		if options == nil {
			options = new(provider.CompleteOptions)
		}

		system, payload, err := buildStreamInput(messages)
		if err != nil {
			yield(nil, err)
			return
		}

		args := []string{
			"--print",
			"--output-format", "stream-json",
			"--input-format", "stream-json",
			"--verbose",
			"--no-session-persistence",
		}

		if c.model != "" {
			args = append(args, "--model", c.model)
		}

		if system != "" {
			args = append(args, "--system-prompt", system)
		}

		cmd := exec.CommandContext(ctx, c.command, args...)
		cmd.Stdin = bytes.NewReader(payload)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			yield(nil, err)
			return
		}

		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			yield(nil, err)
			return
		}

		var (
			messageID string
			modelName = c.model
		)

		stop := func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var env envelope
			if err := json.Unmarshal(line, &env); err != nil {
				continue
			}

			switch env.Type {
			case "system":
				if env.Subtype == "init" && env.Model != "" && modelName == "" {
					modelName = env.Model
				}

			case "assistant":
				if env.Message == nil {
					continue
				}

				if env.Message.ID != "" {
					messageID = env.Message.ID
				}

				if env.Message.Model != "" && env.Message.Model != "<synthetic>" {
					modelName = env.Message.Model
				}

				for _, block := range env.Message.Content {
					var content provider.Content

					switch block.Type {
					case "text":
						if block.Text == "" {
							continue
						}

						content = provider.TextContent(block.Text)

					case "thinking":
						if block.Thinking == "" && block.Signature == "" {
							continue
						}

						content = provider.ReasoningContent(provider.Reasoning{
							Text:      block.Thinking,
							Signature: block.Signature,
						})

					default:
						continue
					}

					delta := &provider.Completion{
						ID:    messageID,
						Model: modelName,

						Message: &provider.Message{
							Role:    provider.MessageRoleAssistant,
							Content: []provider.Content{content},
						},
					}

					if !yield(delta, nil) {
						stop()
						return
					}
				}

			case "result":
				if env.IsError {
					msg := strings.TrimSpace(env.Result)
					if msg == "" {
						msg = "claude cli error"
					}

					yield(nil, &provider.ProviderError{
						Message: msg,
						Err:     errors.New(msg),
					})
					stop()
					return
				}

				final := &provider.Completion{
					ID:    messageID,
					Model: modelName,

					Message: &provider.Message{
						Role: provider.MessageRoleAssistant,
					},

					Status: provider.CompletionStatusCompleted,
				}

				switch env.StopReason {
				case "max_tokens":
					final.Status = provider.CompletionStatusIncomplete
				case "refusal":
					final.Status = provider.CompletionStatusRefused
				}

				if env.Usage != nil {
					final.Usage = &provider.Usage{
						InputTokens:              env.Usage.InputTokens,
						OutputTokens:             env.Usage.OutputTokens,
						CacheReadInputTokens:     env.Usage.CacheReadInputTokens,
						CacheCreationInputTokens: env.Usage.CacheCreationInputTokens,
					}
				}

				if !yield(final, nil) {
					stop()
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			stop()
			yield(nil, err)
			return
		}

		if err := cmd.Wait(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}

			yield(nil, &provider.ProviderError{
				Message: msg,
				Err:     err,
			})
		}
	}
}

func buildStreamInput(messages []provider.Message) (system string, payload []byte, err error) {
	var systemParts []string
	var turns []provider.Message

	for _, m := range messages {
		if m.Role == provider.MessageRoleSystem {
			if strings.TrimSpace(m.Text()) != "" {
				systemParts = append(systemParts, m.Text())
			}
			continue
		}

		turns = append(turns, m)
	}

	lastUser := -1

	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == provider.MessageRoleUser {
			lastUser = i
			break
		}
	}

	var history []provider.Message
	var current provider.Message

	if lastUser >= 0 {
		history = turns[:lastUser]
		current = turns[lastUser]
	} else {
		history = turns
	}

	var sb strings.Builder

	if len(systemParts) > 0 {
		sb.WriteString(strings.Join(systemParts, "\n\n"))
	}

	if len(history) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}

		sb.WriteString("<conversation_history>\n")

		for _, m := range history {
			text := strings.TrimSpace(m.Text())
			if text == "" {
				continue
			}

			role := "user"
			if m.Role == provider.MessageRoleAssistant {
				role = "assistant"
			}

			sb.WriteString("  <turn role=\"")
			sb.WriteString(role)
			sb.WriteString("\">")
			sb.WriteString(text)
			sb.WriteString("</turn>\n")
		}

		sb.WriteString("</conversation_history>")
	}

	system = sb.String()

	var blocks []inputBlock

	if text := strings.TrimSpace(current.Text()); text != "" {
		blocks = append(blocks, inputBlock{Type: "text", Text: current.Text()})
	}

	for _, c := range current.Content {
		if c.File == nil {
			continue
		}

		data := base64.StdEncoding.EncodeToString(c.File.Content)
		mime := c.File.ContentType

		switch mime {
		case "image/jpeg", "image/png", "image/gif", "image/webp":
			blocks = append(blocks, inputBlock{
				Type: "image",
				Source: &inputSource{
					Type:      "base64",
					MediaType: mime,
					Data:      data,
				},
			})

		case "application/pdf":
			blocks = append(blocks, inputBlock{
				Type: "document",
				Source: &inputSource{
					Type:      "base64",
					MediaType: mime,
					Data:      data,
				},
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, inputBlock{Type: "text", Text: ""})
	}

	msg := inputMessage{
		Type: "user",
		Message: inputUser{
			Role:    "user",
			Content: blocks,
		},
	}

	payload, err = json.Marshal(msg)
	if err != nil {
		return "", nil, err
	}

	payload = append(payload, '\n')
	return
}

type inputMessage struct {
	Type    string    `json:"type"`
	Message inputUser `json:"message"`
}

type inputUser struct {
	Role    string       `json:"role"`
	Content []inputBlock `json:"content"`
}

type inputBlock struct {
	Type string `json:"type"`

	Text string `json:"text,omitempty"`

	Source *inputSource `json:"source,omitempty"`
}

type inputSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type envelope struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	Model string `json:"model"`

	Message *cliMessage `json:"message"`

	IsError    bool      `json:"is_error"`
	Result     string    `json:"result"`
	StopReason string    `json:"stop_reason"`
	Usage      *cliUsage `json:"usage"`
}

type cliMessage struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Role  string `json:"role"`

	Content []cliContent `json:"content"`
}

type cliContent struct {
	Type string `json:"type"`

	Text string `json:"text"`

	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

type cliUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`

	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}
