package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"strings"

	"google.golang.org/genai"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/toolid"
	"github.com/adrianliechti/wingman/pkg/provider/tools/computeruse"
	"github.com/adrianliechti/wingman/pkg/provider/tools/custom"
	"github.com/adrianliechti/wingman/pkg/provider/tools/shell"
	"github.com/adrianliechti/wingman/pkg/provider/tools/texteditor"
	"github.com/adrianliechti/wingman/pkg/provider/tools/toolsearch"
)

var _ provider.Completer = (*Completer)(nil)

type Completer struct {
	*Config
}

func NewCompleter(model string, options ...Option) (*Completer, error) {
	cfg := &Config{
		model: model,
	}

	for _, option := range options {
		option(cfg)
	}

	return &Completer{
		Config: cfg,
	}, nil
}

func (c *Completer) Complete(ctx context.Context, messages []provider.Message, options *provider.CompleteOptions) iter.Seq2[*provider.Completion, error] {
	return func(yield func(*provider.Completion, error) bool) {
		messages = provider.ResolveInstructions(messages)
		messages, options = provider.ResolveConfigurationUpdates(messages, options)
		messages, options = toolsearch.Inline(messages, options)

		client, err := c.newClient(ctx)

		if err != nil {
			yield(nil, err)
			return
		}

		contents := convertMessages(messages)

		config, err := convertGenerateConfig(c.model, convertInstruction(messages), options)

		if err != nil {
			yield(nil, err)
			return
		}

		toolAliases := provider.ToolAliases(options.Tools)

		var sawToolCall bool

		for resp, err := range generateContentStream(ctx, client, c.model, contents, config) {
			if err != nil {
				yield(nil, convertError(err))
				return
			}

			delta := &provider.Completion{
				ID: resp.ResponseID,

				Message: &provider.Message{
					Role: provider.MessageRoleAssistant,
				},

				Usage: toCompletionUsage(resp.UsageMetadata),
			}

			if feedback := resp.PromptFeedback; feedback != nil && feedback.BlockReason != "" {
				// A blocked prompt yields no candidates at all.
				delta.StopReason = provider.StopReasonRefusal
				delta.Status = provider.CompletionStatusRefused
				delta.StopDetails = &provider.StopDetails{
					Type:     "refusal",
					Category: string(feedback.BlockReason),
				}
			}

			if len(resp.Candidates) > 0 {
				candidate := resp.Candidates[0]

				// Content is absent when a candidate stops before any output,
				// e.g. on a safety block.
				if candidate.Content != nil {
					delta.Message.Content = toContent(candidate.Content, toolAliases, options.Tools)
				}

				for _, c := range delta.Message.Content {
					if c.ToolCall != nil {
						sawToolCall = true
					}
				}

				applyFinishReason(delta, candidate.FinishReason, sawToolCall)
			}

			if !yield(delta, nil) {
				return
			}
		}
	}
}

func convertInstruction(messages []provider.Message) *genai.Content {
	var parts []*genai.Part

	for _, m := range messages {
		if m.Role != provider.MessageRoleSystem {
			continue
		}

		for _, c := range m.Content {
			if c.Text != "" {
				parts = append(parts, genai.NewPartFromText(c.Text))
			}
		}
	}

	if len(parts) == 0 {
		return nil
	}

	return &genai.Content{
		Parts: parts,
	}
}

// generateContentStream retries once at LOW when the model rejects MINIMAL
// thinking, which several Gemini 3 models (3.1 Pro, 3.7 and 3.8 Flash) do not
// offer. The rejection arrives before any output, so the retry is invisible.
func generateContentStream(ctx context.Context, client *genai.Client, model string, contents []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		for resp, err := range client.Models.GenerateContentStream(ctx, model, contents, config) {
			if err != nil && config.ThinkingConfig != nil && config.ThinkingConfig.ThinkingLevel == genai.ThinkingLevelMinimal && isUnsupportedThinkingLevel(err) {
				config.ThinkingConfig.ThinkingLevel = genai.ThinkingLevelLow

				for resp, err := range client.Models.GenerateContentStream(ctx, model, contents, config) {
					if !yield(resp, err) {
						return
					}
				}

				return
			}

			if !yield(resp, err) {
				return
			}
		}
	}
}

func isUnsupportedThinkingLevel(err error) bool {
	var apierr genai.APIError
	return errors.As(err, &apierr) && apierr.Code == http.StatusBadRequest && strings.Contains(apierr.Message, "Thinking level")
}

func convertThinkingConfig(model string, reasoning *provider.ReasoningOptions) *genai.ThinkingConfig {
	config := &genai.ThinkingConfig{
		IncludeThoughts: reasoning.IncludeSummary && reasoning.Type != provider.ReasoningTypeDisabled,
	}

	effort := reasoning.Effort

	if reasoning.Type == provider.ReasoningTypeDisabled {
		effort = ""
	}

	// Gemini 2.x rejects thinkingLevel and is steered by a token budget.
	if strings.HasPrefix(strings.TrimPrefix(model, "models/"), "gemini-2") {
		switch {
		case reasoning.Type == provider.ReasoningTypeDisabled:
			config.ThinkingBudget = new(int32(0))
		case effort == "":
		case effort == provider.EffortMinimal, effort == provider.EffortLow:
			config.ThinkingBudget = new(int32(1024))
		case effort == provider.EffortMedium:
			config.ThinkingBudget = new(int32(8192))
		default:
			config.ThinkingBudget = new(int32(24576))
		}

		return config
	}

	// Gemini 3 cannot fully disable thinking; minimal is the closest level.
	if reasoning.Type == provider.ReasoningTypeDisabled {
		effort = provider.EffortMinimal
	}

	switch effort {
	case provider.EffortMinimal:
		config.ThinkingLevel = genai.ThinkingLevelMinimal
	case provider.EffortLow:
		config.ThinkingLevel = genai.ThinkingLevelLow
	case provider.EffortMedium:
		config.ThinkingLevel = genai.ThinkingLevelMedium
	case provider.EffortHigh, provider.EffortXHigh, provider.EffortMax:
		config.ThinkingLevel = genai.ThinkingLevelHigh
	}

	return config
}

func convertGenerateConfig(model string, instruction *genai.Content, options *provider.CompleteOptions) (*genai.GenerateContentConfig, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: instruction,
	}

	if options.ReasoningOptions != nil {
		config.ThinkingConfig = convertThinkingConfig(model, options.ReasoningOptions)
	}

	flatTools := provider.FlattenTools(options.Tools)

	if len(flatTools) > 0 {
		tools, err := convertTools(flatTools)

		if err != nil {
			return nil, err
		}

		config.Tools = tools

		fcc := &genai.FunctionCallingConfig{}

		if options.ToolOptions != nil {
			switch options.ToolOptions.Choice {
			case provider.ToolChoiceNone:
				fcc.Mode = genai.FunctionCallingConfigModeNone

			case provider.ToolChoiceAuto:
				fcc.Mode = genai.FunctionCallingConfigModeAuto

			case provider.ToolChoiceAny:
				fcc.Mode = genai.FunctionCallingConfigModeAny
				fcc.AllowedFunctionNames = options.ToolOptions.Allowed
			}
		}

		if fcc.Mode == "" || fcc.Mode == genai.FunctionCallingConfigModeAuto {
			for _, t := range flatTools {
				if t.Strict != nil && *t.Strict {
					fcc.Mode = genai.FunctionCallingConfigModeValidated
					break
				}
			}
		}

		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: fcc,
		}
	}

	if len(options.Stop) > 0 {
		config.StopSequences = options.Stop
	}

	if options.MaxTokens != nil {
		config.MaxOutputTokens = int32(*options.MaxTokens)
	}

	if options.Temperature != nil {
		config.Temperature = options.Temperature
	}

	if options.Schema != nil {
		config.ResponseMIMEType = "application/json"
		config.ResponseJsonSchema = options.Schema.Properties
	}

	return config, nil
}

func convertContent(message provider.Message, toolCallNames map[string]string) *genai.Content {
	content := &genai.Content{}

	switch message.Role {
	case provider.MessageRoleUser:
		content.Role = "user"

		for _, c := range message.Content {
			if text := strings.TrimRight(c.Text, " \t\n\r"); text != "" {
				part := genai.NewPartFromText(text)
				content.Parts = append(content.Parts, part)
			}

			if c.File != nil {
				// Gemini's inline_data accepts any mime; the model decides
				// what it can interpret. Forward as-is.
				part := genai.NewPartFromBytes(c.File.Content, c.File.ContentType)
				content.Parts = append(content.Parts, part)
			}

			if c.ToolResult != nil {
				var (
					textBuilder strings.Builder
					fileParts   []*genai.FunctionResponsePart
				)
				for _, p := range c.ToolResult.Parts {
					if p.Text != "" {
						textBuilder.WriteString(p.Text)
					}
					if p.File != nil {
						fileParts = append(fileParts, &genai.FunctionResponsePart{
							InlineData: &genai.FunctionResponseBlob{
								MIMEType:    p.File.ContentType,
								Data:        p.File.Content,
								DisplayName: p.File.Name,
							},
						})
					}
				}
				text := textBuilder.String()

				parameters := map[string]any{}

				if text != "" {
					var data any
					if err := json.Unmarshal([]byte(text), &data); err == nil {
						switch val := data.(type) {
						case map[string]any:
							parameters = val
						case []any:
							parameters = map[string]any{"data": val}
						default:
							parameters = map[string]any{"output": text}
						}
					} else {
						parameters = map[string]any{"output": text}
					}
				}

				if c.ToolResult.IsError {
					var errorValue any = text
					if existing, ok := parameters["error"]; ok {
						errorValue = existing
					} else if output, ok := parameters["output"]; ok && len(parameters) == 1 {
						errorValue = output
					} else if len(parameters) > 0 {
						errorValue = parameters
					}
					parameters = map[string]any{"error": errorValue}
				}

				id, encodedName, signature := parseToolID(c.ToolResult.ID)

				// Resolve the tool name: prefer the encoded round-trip form
				// (assistant calls Gemini originated), fall back to looking up
				// the matching prior tool call's Name by id. Gemini's
				// FunctionResponse requires a non-empty name on the wire.
				name := encodedName
				if name == "" {
					name = toolCallNames[id]
				}

				part := genai.NewPartFromFunctionResponse(name, parameters)
				part.FunctionResponse.ID = id
				part.FunctionResponse.Parts = fileParts
				part.ThoughtSignature = signature

				content.Parts = append(content.Parts, part)
			}
		}

	case provider.MessageRoleAssistant:
		content.Role = "model"

		var pendingSig []byte

		for _, c := range message.Content {
			if c.Reasoning != nil {
				if c.Reasoning.Text == "" && c.Reasoning.Summary == "" {
					if c.Reasoning.Signature != "" {
						pendingSig = DecodeThoughtSignature(c.Reasoning.Signature)
					}
					continue
				}

				text := c.Reasoning.Text
				if text == "" {
					text = c.Reasoning.Summary
				}

				part := genai.NewPartFromText(text)
				part.Thought = true
				if c.Reasoning.Signature != "" {
					part.ThoughtSignature = DecodeThoughtSignature(c.Reasoning.Signature)
				}
				content.Parts = append(content.Parts, part)
				continue
			}

			if text := strings.TrimRight(c.Text, " \t\n\r"); text != "" {
				part := genai.NewPartFromText(text)
				if pendingSig != nil {
					part.ThoughtSignature = pendingSig
					pendingSig = nil
				}
				content.Parts = append(content.Parts, part)
				continue
			}

			if c.ToolCall != nil {
				arguments := c.ToolCall.Arguments

				if c.ToolCall.Kind == provider.ToolKindCustom && !custom.IsWrapped(arguments) {
					// freeform input from the client — re-encode it the way the
					// emulated tool was declared
					arguments = custom.Wrap(arguments)
				}

				var data map[string]any
				if err := json.Unmarshal([]byte(arguments), &data); err != nil || data == nil {
					data = map[string]any{}
				}

				id, encodedName, signature := parseToolID(c.ToolCall.ID)

				// Prefer the explicit Name field (always set on assistant tool
				// calls); fall back to the encoded suffix for round-tripped IDs.
				name := provider.FlattenToolName(*c.ToolCall)
				if name == "" {
					name = encodedName
				}

				part := genai.NewPartFromFunctionCall(name, data)
				part.FunctionCall.ID = id
				if signature != nil {
					part.ThoughtSignature = signature
				} else if pendingSig != nil {
					part.ThoughtSignature = pendingSig
					pendingSig = nil
				} else {
					part.ThoughtSignature = dummyThoughtSignature
				}

				content.Parts = append(content.Parts, part)
			}
		}

		if pendingSig != nil {
			part := &genai.Part{ThoughtSignature: pendingSig}
			content.Parts = append(content.Parts, part)
		}
	}

	return content
}

func convertMessages(messages []provider.Message) []*genai.Content {
	// Build a callID → name index from assistant tool calls so tool results
	// (which have no Name field) can recover the tool name when the id isn't
	// in encoded form. The lookup uses both the raw id and the plain id
	// component of an encoded id, since clients may replay either shape.
	toolCallNames := map[string]string{}
	for _, m := range messages {
		if m.Role != provider.MessageRoleAssistant {
			continue
		}
		for _, c := range m.Content {
			if c.ToolCall == nil || c.ToolCall.Name == "" {
				continue
			}
			name := provider.FlattenToolName(*c.ToolCall)

			toolCallNames[c.ToolCall.ID] = name
			if plain, _, _ := parseToolID(c.ToolCall.ID); plain != "" && plain != c.ToolCall.ID {
				toolCallNames[plain] = name
			}
		}
	}

	var result []*genai.Content
	for _, m := range messages {
		if m.Role == provider.MessageRoleUser || m.Role == provider.MessageRoleAssistant {
			result = append(result, convertContent(m, toolCallNames))
		}
	}
	return result
}

func convertTools(tools []provider.Tool) ([]*genai.Tool, error) {
	var functions []*genai.FunctionDeclaration

	for _, t := range tools {
		if t.Kind == provider.ToolKindTextEditor {
			t = texteditor.FunctionTool(t)
		}

		if t.Kind == provider.ToolKindComputer {
			t = computeruse.FunctionTool(t)
		}

		if t.Kind == provider.ToolKindShell {
			t = shell.FunctionTool(t)
		}

		if t.Kind == provider.ToolKindCustom {
			// Gemini has no freeform tool type — carry the text in a single
			// string parameter and unwrap it on the way back
			t = custom.FunctionTool(t)
		}

		if t.Kind != provider.ToolKindFunction {
			return nil, provider.UnsupportedToolError(t)
		}

		function := &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,

			ParametersJsonSchema: t.Parameters,
		}

		functions = append(functions, function)
	}

	if len(functions) == 0 {
		return nil, nil
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: functions,
		},
	}, nil
}

func toContent(content *genai.Content, toolAliases map[string]provider.Tool, tools []provider.Tool) []provider.Content {
	var parts []provider.Content

	for _, p := range content.Parts {
		sig := EncodeThoughtSignature(p.ThoughtSignature)

		if p.Thought {
			parts = append(parts, provider.ReasoningContent(provider.Reasoning{
				Text:      p.Text,
				Signature: sig,
			}))
			continue
		}

		if sig != "" && p.FunctionCall == nil {
			parts = append(parts, provider.ReasoningContent(provider.Reasoning{
				ID:        "gemsig_" + generateCallID(),
				Signature: sig,
			}))
		}

		if p.Text != "" {
			parts = append(parts, provider.TextContent(p.Text))
		}

		// Inline media bytes — image-emitting Gemini models (gemini-*-image
		// preview families) return generated images this way.
		if p.InlineData != nil {
			parts = append(parts, provider.FileContent(&provider.File{
				Name:        p.InlineData.DisplayName,
				Content:     p.InlineData.Data,
				ContentType: p.InlineData.MIMEType,
			}))
		}

		// URI-based file reference (e.g. files uploaded via the Files API).
		// The URI is the only thing the upstream stored; pack it into the
		// File.Content per the codebase convention for URI-only references.
		if p.FileData != nil {
			parts = append(parts, provider.FileContent(&provider.File{
				Name:        p.FileData.DisplayName,
				Content:     []byte(p.FileData.FileURI),
				ContentType: p.FileData.MIMEType,
			}))
		}

		if p.FunctionCall != nil {
			args := "{}"

			if len(p.FunctionCall.Args) > 0 {
				data, _ := json.Marshal(p.FunctionCall.Args)
				args = string(data)
			}

			call := provider.UnflattenToolCall(toolAliases, provider.ToolCall{
				ID: formatToolID(p.FunctionCall.ID, p.FunctionCall.Name, p.ThoughtSignature),

				Name:      p.FunctionCall.Name,
				Arguments: args,
			})

			// Gemini delivers complete arguments, so the emulated wrapper can be
			// unwrapped in place — no buffering needed
			if custom.IsEmulated(tools, p.FunctionCall.Name) {
				call.Kind = provider.ToolKindCustom
				call.Arguments = custom.Unwrap(call.Arguments)
			}

			parts = append(parts, provider.ToolCallContent(call))
		}
	}

	return parts
}

// Gemini reports STOP for tool-call turns and may deliver the finish reason in
// a later functionCall-less chunk, so tool use is derived from the streamed
// content instead of the finish reason.
func applyFinishReason(delta *provider.Completion, reason genai.FinishReason, sawToolCall bool) {
	switch reason {
	case genai.FinishReasonStop:
		delta.StopReason = provider.StopReasonEndTurn

		if sawToolCall {
			delta.StopReason = provider.StopReasonToolUse
		}

	case genai.FinishReasonMaxTokens:
		delta.StopReason = provider.StopReasonMaxTokens
		delta.Status = provider.CompletionStatusIncomplete

	case genai.FinishReasonSafety,
		genai.FinishReasonRecitation,
		genai.FinishReasonBlocklist,
		genai.FinishReasonProhibitedContent,
		genai.FinishReasonSPII,
		genai.FinishReasonImageSafety,
		genai.FinishReasonImageProhibitedContent,
		genai.FinishReasonImageRecitation:
		delta.StopReason = provider.StopReasonRefusal
		delta.Status = provider.CompletionStatusRefused
		delta.StopDetails = &provider.StopDetails{
			Type:     "refusal",
			Category: string(reason),
		}

	case genai.FinishReasonLanguage,
		genai.FinishReasonMalformedFunctionCall,
		genai.FinishReasonUnexpectedToolCall,
		genai.FinishReasonTooManyToolCalls,
		genai.FinishReasonNoImage,
		genai.FinishReasonOther,
		genai.FinishReasonImageOther:
		delta.Status = provider.CompletionStatusFailed
		delta.StopDetails = &provider.StopDetails{
			Type: string(reason),
		}
	}
}

func toCompletionUsage(metadata *genai.GenerateContentResponseUsageMetadata) *provider.Usage {
	if metadata == nil {
		return nil
	}
	// The SDK does not retain field presence, so zero could mean an omitted
	// breakdown. Only a positive count establishes measured reasoning usage.
	var reasoningTokens *int
	if metadata.ThoughtsTokenCount > 0 {
		reasoningTokens = new(int(metadata.ThoughtsTokenCount))
	}

	// Gemini's PromptTokenCount already includes cached tokens, and
	// thoughts tokens are reported separately from candidates. Add thoughts
	// to OutputTokens so callers see the full generated-token cost, and expose
	// them as ReasoningTokens (the subset of OutputTokens spent thinking).
	return &provider.Usage{
		InputTokens:  int(metadata.PromptTokenCount),
		OutputTokens: int(metadata.CandidatesTokenCount) + int(metadata.ThoughtsTokenCount),

		ReasoningTokens: reasoningTokens,

		CacheReadInputTokens: int(metadata.CachedContentTokenCount),
	}
}

// formatToolID packs a call id, tool name, and an optional thought signature
// into a single string of the form "id::name::base64sig" so that Gemini-served
// assistant tool calls can round-trip the name back to us when the client
// replays them — Gemini's FunctionResponse requires the name on the wire.
// A blank id is replaced with a freshly-generated one.
func formatToolID(id, name string, signature []byte) string {
	if id == "" {
		id = generateCallID()
	}
	return strings.Join([]string{id, name, base64.StdEncoding.EncodeToString(signature)}, "::")
}

func generateCallID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func StripToolIDSignature(s string) string {
	return toolid.StripSignature(s)
}

// EncodeThoughtSignature renders Gemini's opaque signature bytes as base64,
// the form in which a provider signature travels: the OpenAI and Anthropic
// surfaces place it in JSON strings, which cannot carry arbitrary bytes.
func EncodeThoughtSignature(signature []byte) string {
	if len(signature) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(signature)
}

// DecodeThoughtSignature restores the bytes for the Gemini wire. A value that
// is not base64 is taken as the raw bytes of an older, in-process history.
func DecodeThoughtSignature(signature string) []byte {
	if signature == "" {
		return nil
	}
	if data, err := base64.StdEncoding.DecodeString(signature); err == nil {
		return data
	}
	return []byte(signature)
}

// dummyThoughtSignature bypasses thought-signature validation for tool calls
// migrated from another model or provider.
// https://ai.google.dev/gemini-api/docs/thought-signatures#faqs
var dummyThoughtSignature = []byte("skip_thought_signature_validator")

// parseToolID is the inverse of formatToolID. For plain IDs without "::" the
// entire string is returned as id with empty name and signature.
func parseToolID(s string) (id, name string, signature []byte) {
	parts := strings.SplitN(s, "::", 3)

	if len(parts) > 0 {
		id = parts[0]
	}

	if len(parts) > 1 {
		name = parts[1]
	}

	if len(parts) > 2 {
		signature, _ = base64.StdEncoding.DecodeString(parts[2])
	}

	return
}
