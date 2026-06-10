package google

import (
	"context"
	"net/http"

	"github.com/adrianliechti/wingman/pkg/provider"

	"google.golang.org/genai"
)

type Config struct {
	token string
	model string

	client *http.Client
}

type Option func(*Config)

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

func (c *Config) newClient(ctx context.Context) (*genai.Client, error) {
	client := c.client

	if client == nil {
		client = provider.DefaultClient
	}

	config := &genai.ClientConfig{
		APIKey:  c.token,
		Backend: genai.BackendGeminiAPI,

		HTTPClient: client,
	}

	return genai.NewClient(ctx, config)
}
