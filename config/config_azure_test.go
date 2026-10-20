package config

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func TestAzureSpeechConfiguration(t *testing.T) {
	var synthesisCalls, transcriptionCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "test-key" {
			t.Error("configured token not forwarded")
		}
		switch r.URL.Path {
		case "/tts/cognitiveservices/v1":
			synthesisCalls++
			var ssml struct {
				Voice struct {
					Name string `xml:"name,attr"`
				} `xml:"voice"`
			}
			if err := xml.NewDecoder(r.Body).Decode(&ssml); err != nil {
				t.Error(err)
			}
			if ssml.Voice.Name != "de-DE-Seraphina:DragonHDLatestNeural" {
				t.Errorf("configured voice = %q", ssml.Voice.Name)
			}
			io.WriteString(w, "audio")
		case "/speechtotext/transcriptions:transcribe":
			transcriptionCalls++
			if r.URL.Query().Get("api-version") != "2025-10-15" {
				t.Error("incorrect transcription API version")
			}
			io.WriteString(w, `{"combinedPhrases":[{"text":"Grüezi"}]}`)
		default:
			t.Errorf("unexpected Azure URL: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg, err := parseTestConfig(t, `
providers:
  - type: azure
    url: `+server.URL+`/
    token: test-key
    vars:
      voice: de-DE-Seraphina:DragonHDLatestNeural
    models:
      - azure-tts
      - azure-stt
`)
	if err != nil {
		t.Fatal(err)
	}
	synthesizer, err := cfg.Synthesizer("azure-tts")
	if err != nil {
		t.Fatal(err)
	}
	for result, err := range synthesizer.Synthesize(context.Background(), "Grüezi", nil) {
		if err != nil {
			t.Fatal(err)
		}
		if string(result.Content) != "audio" {
			t.Errorf("audio = %q", result.Content)
		}
	}
	transcriber, err := cfg.Transcriber("azure-stt")
	if err != nil {
		t.Fatal(err)
	}
	for result, err := range transcriber.Transcribe(context.Background(), provider.File{Name: "hello.wav"}, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if result.Text != "Grüezi" {
			t.Errorf("transcription = %q", result.Text)
		}
	}
	if synthesisCalls != 1 || transcriptionCalls != 1 {
		t.Fatalf("synthesis requests = %d, transcription requests = %d", synthesisCalls, transcriptionCalls)
	}
}

func TestAzureSpeechConfigurationRequiresURL(t *testing.T) {
	for _, modelType := range []string{"synthesizer", "transcriber"} {
		t.Run(modelType, func(t *testing.T) {
			_, err := parseTestConfig(t, `
providers:
  - type: azure
    token: test-key
    models:
      speech:
        type: `+modelType+`
`)
			if err == nil || !strings.Contains(err.Error(), "requires an http(s) resource URL") {
				t.Fatalf("error = %v, want a missing resource URL error", err)
			}
		})
	}
}
