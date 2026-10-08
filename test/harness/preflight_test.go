package harness

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkipUnlessConfigured(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		proceed bool
	}{
		{"configured OpenAI model", http.StatusOK, `{"data":[{"id":"claude-haiku-5-5"}]}`, true},
		{"configured Gemini model", http.StatusOK, `{"models":[{"name":"models/claude-haiku-5-5"}]}`, true},
		{"model missing", http.StatusOK, `{"data":[{"id":"other-model"}]}`, false},
		{"empty listing", http.StatusOK, `{"data":[]}`, false},
		{"invalid listing", http.StatusOK, `not json`, false},
		{"unauthorized", http.StatusUnauthorized, `{"error":"unauthorized"}`, false},
		{"unavailable", 0, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("unexpected preflight request: %s, authorization %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			if tc.status == 0 {
				server.Close()
			}
			for range 2 {
				proceeded := false
				t.Run("comparison", func(t *testing.T) {
					SkipUnlessConfigured(t, server.URL+"/v1", "test-key", "claude-haiku-5-5")
					proceeded = true
				})
				if proceeded != tc.proceed {
					t.Fatalf("comparison proceeded = %v, want %v", proceeded, tc.proceed)
				}
			}
			if tc.status != 0 && requests != 1 {
				t.Fatalf("model listing requested %d times, want one cached preflight", requests)
			}
		})
	}
}
