package tavily

import (
	"net/http"
)

type Option func(*Client)

func WithDepth(depth string) Option {
	return func(c *Client) { c.depth = depth }
}

func WithClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}
