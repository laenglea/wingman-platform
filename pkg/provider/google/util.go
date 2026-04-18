package google

import (
	"errors"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"

	"google.golang.org/genai"
)

func convertError(err error) error {
	// The genai SDK returns genai.APIError as a value type.
	var apierr genai.APIError

	if errors.As(err, &apierr) {
		statusCode := apierr.Code

		// Google's API gateway returns 400/INVALID_ARGUMENT for auth errors
		// like invalid or expired API keys. Remap these to 401.
		if isAuthError(apierr) {
			statusCode = http.StatusUnauthorized
		}

		// Prefer the clean, human-readable Message field over Error(),
		// which includes the status code, details, and raw maps.
		message := apierr.Message
		if message == "" {
			message = apierr.Error()
		}

		return &provider.ProviderError{
			StatusCode: statusCode,
			Message:    message,
			Err:        err,
		}
	}

	return err
}

// isAuthError detects authentication-related errors from Google's API gateway.
// These arrive as HTTP 400 but are semantically 401 (invalid credentials).
func isAuthError(apierr genai.APIError) bool {
	for _, detail := range apierr.Details {
		if reason, _ := detail["reason"].(string); reason == "API_KEY_INVALID" {
			return true
		}
	}

	return false
}
