package google

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"

	"google.golang.org/genai"
	"google.golang.org/genai/interactions/models/apierrors"
)

func convertError(err error) error {
	var clientError *apierrors.CreateInteractionClientError
	var serverError *apierrors.CreateInteractionServerError
	var responseError *apierrors.ResponseValidationError
	var interactionError *apierrors.APIError
	switch {
	case errors.As(err, &clientError):
		return interactionProviderError(err, clientError.HTTPMeta.Response.StatusCode, value(clientError.Error_.Code), value(clientError.Error_.Message), "")
	case errors.As(err, &serverError):
		return interactionProviderError(err, serverError.HTTPMeta.Response.StatusCode, value(serverError.Error_.Code), value(serverError.Error_.Message), "")
	case errors.As(err, &responseError):
		return interactionProviderError(err, responseError.StatusCode, "", responseError.Message, responseError.Body)
	case errors.As(err, &interactionError):
		return interactionProviderError(err, interactionError.StatusCode, "", interactionError.Message, interactionError.Body)
	}

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
			Code:    statusCode,
			Type:    apierr.Status,
			Message: message,
			Err:     err,
		}
	}

	return err
}

// Interactions has distinct SDK error types. Its gateway can also return the
// original Google error envelope with a numeric code, which the generated
// Interactions schema (a string code) reports as a response validation error.
func interactionProviderError(err error, status int, code, message, body string) error {
	var response struct {
		Error struct {
			Code    json.RawMessage  `json:"code"`
			Status  string           `json:"status"`
			Message string           `json:"message"`
			Details []map[string]any `json:"details"`
		} `json:"error"`
	}
	var details []map[string]any
	// Some non-2xx streaming failures are already framed as SSE.
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data:") {
			body = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			break
		}
	}
	// Streaming gateway failures can use a one-element JSON array, even
	// though the Interactions SDK expects a single error object.
	var envelopes []json.RawMessage
	if json.Unmarshal([]byte(body), &envelopes) == nil && len(envelopes) > 0 {
		body = string(envelopes[0])
	}
	if json.Unmarshal([]byte(body), &response) == nil {
		if response.Error.Message != "" {
			message = response.Error.Message
		}
		code = response.Error.Status
		if code == "" {
			json.Unmarshal(response.Error.Code, &code)
		}
		details = response.Error.Details
	}
	if status == 0 {
		status = http.StatusBadGateway
	}
	if numeric, e := strconv.Atoi(code); e == nil && numeric >= 400 && numeric < 600 {
		status = numeric
	}
	if code == "API_KEY_INVALID" || strings.Contains(message, "API key not valid") || isAuthError(genai.APIError{Details: details}) {
		status = http.StatusUnauthorized
	}
	if message == "" {
		message = err.Error()
	}
	return &provider.ProviderError{Code: status, Type: code, Message: message, Err: err}
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
