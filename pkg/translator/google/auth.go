package google

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
)

func (c *Client) configureCredentials() error {
	options := &credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
		Client: c.client,
	}

	c.credentials = sync.OnceValues(func() (*auth.Credentials, error) {
		return credentials.DetectDefault(options)
	})

	if c.token == "" {
		return nil
	}

	info, err := os.Stat(c.token)

	if err != nil {
		isPath := filepath.IsAbs(c.token) || strings.ContainsAny(c.token, "/\\") || strings.HasSuffix(strings.ToLower(c.token), ".json")

		if !os.IsNotExist(err) || isPath {
			return fmt.Errorf("google translator: open credential file %q: %w", c.token, err)
		}

		return nil
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("google translator: credential path %q is not a regular file", c.token)
	}

	creds, err := credentials.NewCredentialsFromFile(credentials.ServiceAccount, c.token, options)

	if err != nil {
		return fmt.Errorf("google translator: load service account %q: %w", c.token, err)
	}

	c.token = ""
	c.credentials = func() (*auth.Credentials, error) { return creds, nil }

	return nil
}
