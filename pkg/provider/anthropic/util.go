package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/internal/claude"

	"github.com/anthropics/anthropic-sdk-go"
)

func convertError(err error) error {
	var apierr *anthropic.Error

	if errors.As(err, &apierr) {
		message, errType := extractAnthropicErrorInfo(apierr)

		provErr := &provider.ProviderError{
			Code:    apierr.StatusCode,
			Type:    errType,
			Message: message,
			Err:     err,
		}

		if apierr.Response != nil {
			h := apierr.Response.Header
			provErr.RetryAfter = parseRetryAfter(h)
		}

		return provErr
	}

	return err
}

// extractAnthropicErrorInfo pulls the clean error.message and error.type out
// of the SDK's raw JSON body (shape: {"error":{"type":"rate_limit_error",
// "message":"..."}}). The SDK's Error.Error() string includes the HTTP
// method, URL, status, Request-ID, and the raw body — too noisy to surface
// as-is to API clients. Falls back to apierr.Error() if parsing fails.
func extractAnthropicErrorInfo(apierr *anthropic.Error) (message, errType string) {
	raw := apierr.RawJSON()

	if raw != "" {
		var payload struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(raw), &payload); err == nil {
			message = strings.TrimSpace(payload.Error.Message)
			errType = strings.TrimSpace(payload.Error.Type)
		}
	}

	if message == "" {
		message = apierr.Error()
	}

	return message, errType
}

// parseRetryAfter parses Retry-After (seconds, float, HTTP-date).
func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}

	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}

	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		return time.Duration(secs * float64(time.Second))
	}

	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}

	return 0
}

// jsonModeInstruction emulates schema-less JSON mode, which the Messages API
// does not offer natively.
const jsonModeInstruction = "Respond with a single valid JSON object only. Do not wrap it in markdown code fences and do not add any text before or after the JSON."

var LegacyModels = []string{
	"claude-3",

	"sonnet-4-0",
	"sonnet-4-5",

	"opus-4-0",
	"opus-4-1",
	"opus-4-5",

	"haiku-4-5",
}

func outputEffort(e provider.Effort) anthropic.BetaOutputConfigEffort {
	return anthropic.BetaOutputConfigEffort(claude.OutputEffort(e))
}

// progressNotes reports whether returned thinking text holds only the notes
// the model writes between tool calls: display "updates" and between_tools
// return those notes and never reasoning.
func progressNotes(t anthropic.BetaThinkingConfigParamUnion) bool {
	return t.OfBetweenTools != nil || (t.OfAdaptive != nil && t.OfAdaptive.Display == anthropic.BetaThinkingConfigAdaptiveDisplayUpdates)
}

// thinkingReasoning returns progress notes as a summary, so frontends show
// them without the caller asking for reasoning summaries.
func thinkingReasoning(text, signature string, notes bool) provider.Reasoning {
	if notes {
		return provider.Reasoning{Summary: text, Signature: signature}
	}
	return provider.Reasoning{Text: text, Signature: signature}
}

// thinking is the resolved thinking configuration for one request.
type thinking struct {
	Enabled    bool
	Disabled   bool
	Summarized bool
	Updates    bool
	Effort     anthropic.BetaOutputConfigEffort
}

// resolveThinking maps the reasoning options onto the Messages API. forced
// marks a request that forces a tool call, which thinking is incompatible
// with.
func (c *Completer) resolveThinking(options *provider.CompleteOptions, forced bool) thinking {
	if claude.MatchesModel(c.model, LegacyModels) {
		return thinking{}
	}
	resolved := claude.ResolveThinking(c.model, options.ReasoningOptions, forced)
	return thinking{
		Enabled:    resolved.Enabled,
		Disabled:   resolved.Disabled,
		Summarized: resolved.Summarized,
		Updates:    resolved.Updates,
		Effort:     anthropic.BetaOutputConfigEffort(resolved.Effort),
	}
}
