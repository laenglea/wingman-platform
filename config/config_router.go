package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adrianliechti/wingman/pkg/otel"
	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/router"
	"github.com/adrianliechti/wingman/pkg/router/adaptive"
	"github.com/adrianliechti/wingman/pkg/router/roundrobin"
)

type routerConfig struct {
	Type string `yaml:"type"`

	Models   []string `yaml:"models"`
	Fallback string   `yaml:"fallback"`

	// FirstTokenTimeout bounds the wait for the first response token before
	// failing over to another provider (e.g. "30s"). Defaults to 2m
	FirstTokenTimeout string `yaml:"first_token_timeout"`

	// FailureThreshold is the number of consecutive failures that open a
	// provider's circuit. Defaults to 5
	FailureThreshold int `yaml:"failure_threshold"`

	// RecoveryTimeout is how long an open circuit waits before allowing a
	// probe request (e.g. "1m"). Defaults to 30s
	RecoveryTimeout string `yaml:"recovery_timeout"`
}

type routerContext struct {
	Completers []provider.Completer
	Fallback   provider.Completer
}

func (cfg *Config) registerRouters(f *configFile) error {
	var configs map[string]routerConfig

	if err := f.Routers.Decode(&configs); err != nil {
		return err
	}

	for _, node := range f.Routers.Content {
		id := node.Value

		config, ok := configs[node.Value]

		if !ok {
			continue
		}

		context := routerContext{}

		for _, m := range config.Models {
			completer, err := cfg.Completer(m)

			if err != nil {
				return err
			}

			context.Completers = append(context.Completers, completer)
		}

		if config.Fallback != "" {
			fallback, err := cfg.Completer(config.Fallback)

			if err != nil {
				return err
			}

			context.Fallback = fallback
		}

		completer, err := createRouter(config, context)

		if err != nil {
			return err
		}

		cfg.RegisterCompleter(id, otel.NewCompleterSpan("router "+id, completer))
	}

	return nil
}

func createRouter(cfg routerConfig, context routerContext) (provider.Completer, error) {
	options, err := routerOptions(cfg, context)

	if err != nil {
		return nil, err
	}

	switch strings.ToLower(cfg.Type) {
	case "roundrobin":
		return roundrobin.NewCompleter(context.Completers, options...)

	case "adaptive":
		return adaptive.NewCompleter(context.Completers, options...)

	default:
		return nil, errors.New("invalid router type: " + cfg.Type)
	}
}

func routerOptions(cfg routerConfig, context routerContext) ([]router.Option, error) {
	var options []router.Option

	if context.Fallback != nil {
		options = append(options, router.WithFallback(context.Fallback))
	}

	if cfg.FirstTokenTimeout != "" {
		timeout, err := parseTimeout("first_token_timeout", cfg.FirstTokenTimeout)

		if err != nil {
			return nil, err
		}

		options = append(options, router.WithFirstTokenTimeout(timeout))
	}

	if cfg.FailureThreshold < 0 {
		return nil, errors.New("invalid failure_threshold: must not be negative")
	}

	if cfg.FailureThreshold > 0 {
		options = append(options, router.WithFailureThreshold(cfg.FailureThreshold))
	}

	if cfg.RecoveryTimeout != "" {
		timeout, err := parseTimeout("recovery_timeout", cfg.RecoveryTimeout)

		if err != nil {
			return nil, err
		}

		options = append(options, router.WithRecoveryTimeout(timeout))
	}

	return options, nil
}

func parseTimeout(name, value string) (time.Duration, error) {
	timeout, err := time.ParseDuration(value)

	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}

	if timeout < 0 {
		return 0, fmt.Errorf("invalid %s: must not be negative", name)
	}

	return timeout, nil
}
