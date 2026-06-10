package openrouter

import (
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"
)

type Config struct {
	url string

	token string
	model string

	client *http.Client
}

type Option func(*Config)

func newConfig(model string, options ...Option) *Config {
	cfg := &Config{
		url:   "https://openrouter.ai/api/v1",
		model: model,
	}

	for _, option := range options {
		option(cfg)
	}

	if cfg.client == nil {
		cfg.client = provider.DefaultClient
	}

	return cfg
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
