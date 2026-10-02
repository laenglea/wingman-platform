package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/httptransport"
	cloudtranslate "cloud.google.com/go/translate/apiv3"
	"google.golang.org/api/option"
)

func (c *Client) translationClient(ctx context.Context) (*cloudtranslate.TranslationClient, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil, "", errors.New("google translator: client is closed")
	}

	if c.sdk != nil {
		return c.sdk, c.parent, nil
	}

	var creds *auth.Credentials

	if c.tokenProvider != nil {
		creds = auth.NewCredentials(&auth.CredentialsOptions{TokenProvider: checkedTokenProvider{c.tokenProvider}})
	} else {
		var err error
		creds, err = c.credentials()

		if err != nil {
			return nil, "", fmt.Errorf("google translator: v3 translation requires Application Default Credentials or a service account JSON token; API keys only support v2 text: %w", err)
		}
	}

	project := c.project

	if project == "" {
		var err error
		project, err = creds.ProjectID(ctx)

		if err != nil {
			return nil, "", fmt.Errorf("google translator: detect translation project: %w", err)
		}
	}

	if project == "" {
		return nil, "", errors.New("google translator: v3 translation requires a Google Cloud project (vars.project)")
	}

	if strings.ContainsAny(project, "/?#") || strings.ContainsAny(c.location, "/?#") {
		return nil, "", errors.New("google translator: invalid translation project or location")
	}

	base := c.client.Transport

	if base == nil {
		base = http.DefaultTransport
	}

	authenticated, err := httptransport.NewClient(&httptransport.Options{
		Credentials:      creds,
		BaseRoundTripper: base,
		Headers:          http.Header{"X-Goog-User-Project": []string{project}},
	})

	if err != nil {
		return nil, "", err
	}

	// The SDK expects an authenticated HTTP client when WithHTTPClient is used.
	// Copy the configured client to preserve its timeout, redirects, and cookie jar.
	client := *c.client
	client.Transport = authenticated.Transport

	c.sdk, err = cloudtranslate.NewTranslationRESTClient(ctx,
		option.WithEndpoint(strings.TrimRight(c.url, "/")),
		option.WithHTTPClient(&client),
	)

	if err != nil {
		return nil, "", err
	}

	c.parent = "projects/" + project + "/locations/" + c.location

	return c.sdk, c.parent, nil
}

// Close releases the Google SDK client after all translations have completed.
func (c *Client) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.sdk != nil {
		return c.sdk.Close()
	}

	return nil
}

type checkedTokenProvider struct {
	auth.TokenProvider
}

func (p checkedTokenProvider) Token(ctx context.Context) (*auth.Token, error) {
	token, err := p.TokenProvider.Token(ctx)

	if err != nil {
		return nil, err
	}

	if token == nil || token.Value == "" {
		return nil, errors.New("google translator: authentication returned an empty access token")
	}

	return token, nil
}
