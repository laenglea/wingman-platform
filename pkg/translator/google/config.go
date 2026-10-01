package google

import (
	"net/http"

	"cloud.google.com/go/auth"
)

type Option func(*Client)

func WithClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithToken accepts an API key for text translation or the path to a service
// account JSON file for OAuth authentication of both text and documents.
// Without a token, the client uses Application Default Credentials.
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// WithProject sets the Google Cloud project used for v3 translation.
// If omitted, the project is detected from the service account file or ADC.
func WithProject(project string) Option {
	return func(c *Client) {
		c.project = project
	}
}

func WithLocation(location string) Option {
	return func(c *Client) {
		if location != "" {
			c.location = location
		}
	}
}

// WithTokenProvider supplies OAuth tokens for translation.
// If omitted, the client uses Application Default Credentials.
func WithTokenProvider(provider auth.TokenProvider) Option {
	return func(c *Client) {
		c.tokenProvider = provider
	}
}
