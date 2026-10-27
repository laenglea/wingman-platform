package protocol_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveNativeErrorParity(t *testing.T) {
	referenceLive(t)
	local := gateway(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request reached the upstream")
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := &http.Client{Timeout: 30 * time.Second}
	for _, p := range protocols(false) {
		for _, scenario := range []string{"malformed-json", "unknown-model"} {
			t.Run(p.name+"/"+scenario, func(t *testing.T) {
				p := liveRequest(t, p, "wingman-nonexistent-reference-model", "text")
				raw := []byte(`{"model":`)
				if scenario == "unknown-model" {
					var err error
					raw, err = json.Marshal(p.body)
					if err != nil {
						t.Fatal(err)
					}
				}
				base := "https://api.openai.com"
				switch p.name {
				case "anthropic":
					base = "https://api.anthropic.com"
				case "gemini":
					base = "https://generativelanguage.googleapis.com"
				}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+p.path, bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				switch p.name {
				case "chat", "responses":
					req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
				case "anthropic":
					req.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))
					req.Header.Set("anthropic-version", "2023-06-01")
				case "gemini":
					req.Header.Set("x-goog-api-key", os.Getenv("GEMINI_API_KEY"))
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				native, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				wantStatus := http.StatusBadRequest
				if scenario == "unknown-model" {
					wantStatus = http.StatusNotFound
				}
				requireStatus(t, resp.StatusCode, wantStatus, native)
				status, headers, body := request(t, local, http.MethodPost, p.path, "application/json", bytes.NewReader(raw))
				requireStatus(t, status, resp.StatusCode, body)
				requireType(t, headers, "application/json")
				got, want := object(t, body), object(t, native)
				field := "type"
				if p.name == "gemini" {
					field = "status"
					equal(t, "error.code", at(got, "error", "code"), at(want, "error", "code"))
				}
				equal(t, "error."+field, at(got, "error", field), at(want, "error", field))
				if p.name == "anthropic" {
					equal(t, "type", got["type"], want["type"])
				}
				if scenario == "unknown-model" && (p.name == "chat" || p.name == "responses") {
					equal(t, "error.code", at(got, "error", "code"), at(want, "error", "code"))
				}
				if strings.TrimSpace(textValue(at(got, "error", "message"))) == "" {
					t.Fatal("missing error message")
				}
			})
		}
	}
}
