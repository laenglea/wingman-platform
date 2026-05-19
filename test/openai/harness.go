package openai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/test/harness"

	"github.com/joho/godotenv"
)

const (
	DefaultWingmanURL = "http://localhost:8080/v1"
	DefaultOpenAIURL  = "https://api.openai.com/v1"
)

type ModelCapabilities struct {
	Reasoning  bool
	Compaction bool
	TextEditor bool
}

type Model struct {
	Name         string
	Capabilities ModelCapabilities
}

type Harness struct {
	Wingman harness.Endpoint
	OpenAI  harness.Endpoint
	Client  *harness.Client

	ReferenceModel string
}

func New(t *testing.T) *Harness {
	t.Helper()
	loadDotenv()

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set — skipping comparison tests")
	}

	return &Harness{
		Wingman:        harness.Endpoint{Name: "wingman", BaseURL: env("WINGMAN_BASE_URL", DefaultWingmanURL), APIKey: env("WINGMAN_API_KEY", "test-key")},
		OpenAI:         harness.Endpoint{Name: "openai", BaseURL: env("OPENAI_BASE_URL", DefaultOpenAIURL), APIKey: key},
		Client:         harness.NewClient(),
		ReferenceModel: env("TEST_OPENAI_REFERENCE_MODEL", "gpt-5.4-mini"),
	}
}

func DefaultModels() []Model {
	names := []string{"claude-sonnet-4-6"}
	if v := os.Getenv("TEST_OPENAI_MODELS"); v != "" {
		names = names[:0]
		for s := range strings.SplitSeq(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				names = append(names, s)
			}
		}
	}

	models := make([]Model, len(names))
	for i, name := range names {
		models[i] = Model{Name: name, Capabilities: knownCapabilities(name)}
	}
	return models
}

func knownCapabilities(name string) ModelCapabilities {
	switch name {
	case "gpt-5.4-mini", "gpt-5.4":
		return ModelCapabilities{Reasoning: true, Compaction: true, TextEditor: true}
	case "claude-sonnet-4-6", "claude-opus-4-6", "claude-opus-4-7":
		return ModelCapabilities{Reasoning: true}
	}
	return ModelCapabilities{Reasoning: true}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotenv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}
