package anthropic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrianliechti/wingman/test/harness"

	"github.com/joho/godotenv"
)

const (
	DefaultWingmanURL       = "http://localhost:8080/v1"
	DefaultAnthropicURL     = "https://api.anthropic.com/v1"
	DefaultAnthropicVersion = "2023-06-01"
)

type Harness struct {
	Wingman   harness.Endpoint
	Anthropic harness.Endpoint
	Client    *harness.Client

	ReferenceModel string
}

func New(t *testing.T) *Harness {
	t.Helper()
	loadDotenv()

	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping comparison tests")
	}

	return &Harness{
		Wingman:        harness.Endpoint{Name: "wingman", BaseURL: env("WINGMAN_BASE_URL", DefaultWingmanURL), APIKey: env("WINGMAN_API_KEY", "test-key")},
		Anthropic:      harness.Endpoint{Name: "anthropic", BaseURL: env("ANTHROPIC_BASE_URL", DefaultAnthropicURL), APIKey: key},
		Client:         harness.NewClient(),
		ReferenceModel: env("TEST_ANTHROPIC_REFERENCE_MODEL", "claude-sonnet-4-6"),
	}
}

type ModelCapabilities struct {
	Thinking    bool
	TextEditor  bool
	Compaction  bool
	ComputerUse bool
}

type Model struct {
	Name         string
	Capabilities ModelCapabilities
}

func DefaultModels() []Model {
	names := []string{"claude-sonnet-4-6"}
	if v := os.Getenv("TEST_ANTHROPIC_MODELS"); v != "" {
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
	case "claude-sonnet-4-6", "claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8":
		return ModelCapabilities{Thinking: true, TextEditor: true, Compaction: true, ComputerUse: true}
	case "claude-sonnet-4-5", "claude-opus-4-5":
		return ModelCapabilities{Thinking: true, TextEditor: true, ComputerUse: true}
	case "claude-haiku-4-5":
		return ModelCapabilities{}
	}
	return ModelCapabilities{Thinking: true}
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
