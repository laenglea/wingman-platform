package azure

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

const defaultVoice = "en-US-Ava:DragonHDLatestNeural"

type Config struct {
	url   string
	voice string

	token string
	model string

	client *http.Client
}

type Option func(*Config)

func newConfig(endpoint, model string, options ...Option) (*Config, error) {
	cfg := &Config{
		url:    strings.TrimSpace(endpoint),
		model:  model,
		voice:  defaultVoice,
		client: provider.DefaultClient,
	}

	for _, option := range options {
		option(cfg)
	}

	resourceURL, err := url.Parse(cfg.url)
	if err != nil || (resourceURL.Scheme != "http" && resourceURL.Scheme != "https") || resourceURL.Hostname() == "" ||
		resourceURL.User != nil || (resourceURL.Path != "" && resourceURL.Path != "/") || resourceURL.RawQuery != "" || resourceURL.ForceQuery || resourceURL.Fragment != "" {
		return nil, errors.New("azure speech requires an http(s) resource URL without a path, query, fragment, or credentials (e.g. https://your-resource.cognitiveservices.azure.com)")
	}
	cfg.url = resourceURL.Scheme + "://" + resourceURL.Host

	return cfg, nil
}

// WithVoice sets the voice used when a synthesis request doesn't specify one.
func WithVoice(voice string) Option {
	return func(c *Config) {
		c.voice = voice
	}
}

func WithClient(client *http.Client) Option {
	return func(c *Config) {
		c.client = client
	}
}

func WithToken(token string) Option {
	return func(c *Config) {
		c.token = token
	}
}
