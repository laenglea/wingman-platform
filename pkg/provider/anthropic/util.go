package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"

	"github.com/anthropics/anthropic-sdk-go"
)

func convertError(err error) error {
	var apierr *anthropic.Error

	if errors.As(err, &apierr) {
		provErr := &provider.ProviderError{
			StatusCode: apierr.StatusCode,
			Message:    extractAnthropicMessage(apierr),
			Err:        err,
		}

		if apierr.Response != nil {
			h := apierr.Response.Header
			provErr.RetryAfter = parseRetryAfter(h)
		}

		return provErr
	}

	return err
}

// extractAnthropicMessage pulls the clean error.message out of the SDK's
// raw JSON body. The SDK's Error.Error() string includes the HTTP method,
// URL, status, Request-ID, and the raw body — too noisy to surface as-is
// to API clients. Falls back to apierr.Error() if parsing fails.
func extractAnthropicMessage(apierr *anthropic.Error) string {
	raw := apierr.RawJSON()

	if raw != "" {
		var payload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(raw), &payload); err == nil {
			if msg := strings.TrimSpace(payload.Error.Message); msg != "" {
				return msg
			}
		}
	}

	return apierr.Error()
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

func isAdaptiveThinkingModel(model string) bool {
	model = strings.ToLower(model)

	thinkingPatterns := []string{
		"sonnet-4-6",

		"opus-4-7",
		"opus-4-6",
	}

	for _, p := range thinkingPatterns {
		if strings.Contains(model, p) {
			return true
		}
	}

	return false
}
