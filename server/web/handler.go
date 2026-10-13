package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const maxBodyBytes = 1 << 20

func IsTavily(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/v1/tavily/") }
func IsExa(r *http.Request) bool    { return strings.HasPrefix(r.URL.Path, "/v1/exa/") }

// Credentials normalizes SDK credentials before Wingman's existing authorizers.
// An explicit Authorization header always takes precedence.
func Credentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			key := ""
			if IsExa(r) {
				key = r.Header.Get("x-api-key")
			} else if IsTavily(r) && r.Body != nil {
				body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
				r.Body.Close()
				if err != nil {
					WriteError(w, r, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				var credentials struct {
					APIKey string `json:"api_key"`
				}
				if json.Unmarshal(body, &credentials) == nil {
					key = credentials.APIKey
				}
			}
			if key != "" {
				r = r.Clone(r.Context())
				r.Header.Set("Authorization", "Bearer "+key)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, message string) {
	var body any = map[string]any{"error": message}
	if IsTavily(r) {
		body = map[string]any{"detail": map[string]string{"error": message}}
	} else if IsExa(r) {
		tag := "INVALID_REQUEST"
		switch status {
		case http.StatusUnauthorized:
			tag = "UNAUTHORIZED"
		case http.StatusNotFound:
			tag = "NOT_FOUND"
		case http.StatusBadGateway:
			tag = "UPSTREAM_ERROR"
		}
		body = map[string]string{"error": message, "tag": tag, "requestId": uuid.NewString()}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func WriteJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// SDKs sometimes send disabled options explicitly. Accept only the documented
// default of an unsupported option, so requested behavior is never dropped.
func ReadRequest(w http.ResponseWriter, r *http.Request, target any, defaults map[string]any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return errors.New("invalid JSON request")
	}
	if fields == nil {
		return errors.New("request must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	for name, expected := range defaults {
		value, ok := fields[name]
		if !ok {
			continue
		}
		want, _ := json.Marshal(expected)
		if string(value) != "null" && !bytes.Equal(bytes.TrimSpace(value), want) {
			return fmt.Errorf("%s is unsupported; omit it or use %s", name, want)
		}
		delete(fields, name)
	}
	body, _ := json.Marshal(fields)
	decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}

type Selection struct {
	Model  string `json:"model"`
	APIKey string `json:"api_key"`
}

func (s Selection) Provider(r *http.Request) (string, error) {
	model := r.URL.Query().Get("model")
	if s.Model != "" {
		if model != "" && model != s.Model {
			return "", errors.New("conflicting model selections")
		}
		model = s.Model
	}
	return model, nil
}
