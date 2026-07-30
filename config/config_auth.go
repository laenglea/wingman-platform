package config

import (
	"errors"
	"strings"

	"github.com/adrianliechti/wingman/pkg/auth"
	"github.com/adrianliechti/wingman/pkg/auth/obo"
)

type authConfig struct {
	Type string `yaml:"type"`

	// static
	Token string `yaml:"token"`

	// obo
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Scope        string `yaml:"scope"`
}

// createClientAuth builds the outbound auth (token exchanger) for an MCP or tool
// connection. It returns nil when no auth block is configured.
func createClientAuth(cfg *authConfig) (auth.TokenExchanger, error) {
	if cfg == nil {
		return nil, nil
	}

	switch strings.ToLower(cfg.Type) {
	case "obo":
		exchanger, err := obo.New(cfg.Issuer, cfg.ClientID, cfg.ClientSecret, cfg.Scope)

		if err != nil {
			return nil, err
		}

		return exchanger, nil

	case "static":
		if cfg.Token == "" {
			return nil, errors.New("static auth: token is required")
		}

		return auth.NewStaticExchanger(cfg.Token), nil

	case "passthrough":
		return auth.PassthroughExchanger{}, nil

	default:
		return nil, errors.New("invalid auth type: " + cfg.Type)
	}
}
