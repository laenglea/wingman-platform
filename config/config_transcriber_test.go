package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTranscriberProviderConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
providers:
  - type: gemini
    token: test-gemini-token
    models:
      - gemini-3.5-transcribe

  - type: google
    token: test-gemini-token
    models:
      gemini-stt:
        id: gemini-3.5-transcribe
        type: transcriber
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("parse transcriber provider example: %v", err)
	}

	for _, model := range []string{"gemini-3.5-transcribe", "gemini-stt"} {
		transcriber, err := cfg.Transcriber(model)
		if err != nil {
			t.Fatalf("transcriber %q: %v", model, err)
		}

		if transcriber == nil {
			t.Errorf("transcriber %q: expected an adapter, got nil", model)
		}
	}
}
