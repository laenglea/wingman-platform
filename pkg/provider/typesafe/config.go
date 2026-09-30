package typesafe

import "net/http"

type Config struct {
	endpoint string
	model    string
	token    string

	client     *http.Client
	maxRetries int
}

type Option func(*Config)

func WithClient(client *http.Client) Option {
	return func(c *Config) { c.client = client }
}

func WithToken(token string) Option {
	return func(c *Config) { c.token = token }
}

func WithMaxRetries(retries int) Option {
	return func(c *Config) { c.maxRetries = max(0, retries) }
}
