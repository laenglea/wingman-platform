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
	code := provider.CodeFromError(err, 0)
	return errorTypeFromCode(code)
}

func errorTypeFromCode(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "authentication_error"
	case code == http.StatusForbidden:
		return "permission_error"
	case code == http.StatusNotFound:
		return "not_found_error"
	case code == http.StatusRequestTimeout:
		return "timeout_error"
	case code == http.StatusConflict:
		return "conflict_error"
	case code == http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case code == http.StatusServiceUnavailable:
		return "overloaded_error"
	case code >= 500:
		return "server_error"
	case code >= 400:
		return "invalid_request_error"
	default:
		return "server_error"
	}
}

func WriteError(w http.ResponseWriter, code int, err error) {
	// Use real status code from upstream provider if available
	code = provider.CodeFromError(err, code)

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
			Code:    provider.TypeFromError(err),
			Message: err.Error(),
		},
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	enc.Encode(resp)
}
