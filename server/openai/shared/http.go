package shared

import (
	"encoding/json"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func WriteJson(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	enc.Encode(v)
}

// ErrorTypeFromError maps a ProviderError to an OpenAI-compatible error type string.
func ErrorTypeFromError(err error) string {
	code := provider.StatusCodeFromError(err, 0)
	return errorTypeFromCode(code)
}

func errorTypeFromCode(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "authentication_error"
	case code == http.StatusForbidden:
		return "permission_error"
	case code == http.StatusNotFound:
		return "not_found"
	case code == http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case code >= 500:
		return "server_error"
	case code >= 400:
		return "invalid_request"
	default:
		return "server_error"
	}
}

func WriteError(w http.ResponseWriter, code int, err error) {
	// Use real status code from upstream provider if available
	code = provider.StatusCodeFromError(err, code)

	// Propagate Retry-After from upstream
	if v := provider.RetryAfterHeaderValue(provider.RetryAfterFromError(err)); v != "" {
		w.Header().Set("Retry-After", v)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	errorType := errorTypeFromCode(code)

	resp := ErrorResponse{
		Error: Error{
			Type:    errorType,
			Message: err.Error(),
		},
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	enc.Encode(resp)
}
