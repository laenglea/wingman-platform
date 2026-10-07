package config

import (
	"errors"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/adapter/decider"
	"github.com/adrianliechti/wingman/pkg/provider/openai"
	"github.com/adrianliechti/wingman/pkg/provider/typesafe"
)

func (cfg *Config) RegisterDecider(id string, p provider.Decider) {
	cfg.RegisterModel(id)
	if cfg.decider == nil {
		cfg.decider = make(map[string]provider.Decider)
	}
	cfg.decider[id] = p
}

func (cfg *Config) Decider(id string) (provider.Decider, error) {
	if p, ok := cfg.decider[id]; ok {
		return p, nil
	}
	if p, err := cfg.Completer(id); err == nil {
		return decider.FromCompleter(id, p), nil
	}
	if p, err := cfg.Embedder(id); err == nil {
		return decider.FromEmbedder(id, p), nil
	}
	return nil, errors.New("decision model not found: " + id)
}

func createDecider(cfg providerConfig, model modelContext) (provider.Decider, error) {
	if strings.EqualFold(cfg.Type, "openai") || strings.EqualFold(cfg.Type, "openai-compatible") {
		var options []openai.Option
		if cfg.Token != "" {
			options = append(options, openai.WithToken(cfg.Token))
		}
		if model.Client != nil {
			options = append(options, openai.WithClient(model.Client))
		}
		if model.MaxRetries != nil {
			options = append(options, openai.WithMaxRetries(*model.MaxRetries))
		}
		return openai.NewDecider(cfg.URL, model.ID, options...)
	}
	if !strings.EqualFold(cfg.Type, "typesafe") {
		return nil, errors.New("invalid decider type: " + cfg.Type)
	}
	var options []typesafe.Option
	if cfg.Token != "" {
		options = append(options, typesafe.WithToken(cfg.Token))
	}
	if model.Client != nil {
		options = append(options, typesafe.WithClient(model.Client))
	}
	if model.MaxRetries != nil {
		options = append(options, typesafe.WithMaxRetries(*model.MaxRetries))
	}
	return typesafe.NewDecider(cfg.URL, model.ID, options...)
}
