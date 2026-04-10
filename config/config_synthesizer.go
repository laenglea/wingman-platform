package config

import (
	"errors"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/azurespeech"
	"github.com/adrianliechti/wingman/pkg/provider/openai"
	"github.com/adrianliechti/wingman/pkg/provider/openrouter"
	"github.com/adrianliechti/wingman/pkg/provider/xai"
)

func (cfg *Config) RegisterSynthesizer(id string, p provider.Synthesizer) {
	cfg.RegisterModel(id)

	if cfg.synthesizer == nil {
		cfg.synthesizer = make(map[string]provider.Synthesizer)
	}

	if _, ok := cfg.synthesizer[""]; !ok {
		cfg.synthesizer[""] = p
	}

	cfg.synthesizer[id] = p
}

func (cfg *Config) Synthesizer(id string) (provider.Synthesizer, error) {
	if cfg.synthesizer != nil {
		if s, ok := cfg.synthesizer[id]; ok {
			return s, nil
		}
	}

	return nil, errors.New("synthesizer not found: " + id)
}

func createSynthesizer(cfg providerConfig, model modelContext) (provider.Synthesizer, error) {
	switch strings.ToLower(cfg.Type) {
	case "openai", "openai-compatible":
		return openaiSynthesizer(cfg, model)

	case "openrouter":
		return openrouterSynthesizer(cfg, model)

	case "azurespeech", "azure-speech":
		return azureSpeechSynthesizer(cfg, model)

	case "xai":
		return xaiSynthesizer(cfg, model)

	default:
		return nil, errors.New("invalid synthesizer type: " + cfg.Type)
	}
}

func azureSpeechSynthesizer(cfg providerConfig, model modelContext) (provider.Synthesizer, error) {
	var options []azurespeech.Option

	if cfg.Token != "" {
		options = append(options, azurespeech.WithToken(cfg.Token))
	}

	if model.Client != nil {
		options = append(options, azurespeech.WithClient(model.Client))
	}

	region := cfg.Vars["region"]

	return azurespeech.NewSynthesizer(region, model.ID, options...)
}

func openaiSynthesizer(cfg providerConfig, model modelContext) (provider.Synthesizer, error) {
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

	return openai.NewSynthesizer(cfg.URL, model.ID, options...)
}

func xaiSynthesizer(cfg providerConfig, model modelContext) (provider.Synthesizer, error) {
	var options []xai.Option

	if cfg.Token != "" {
		options = append(options, xai.WithToken(cfg.Token))
	}

	if model.Client != nil {
		options = append(options, xai.WithClient(model.Client))
	}

	return xai.NewSynthesizer(model.ID, options...)
}

func openrouterSynthesizer(cfg providerConfig, model modelContext) (provider.Synthesizer, error) {
	var options []openrouter.Option

	if cfg.Token != "" {
		options = append(options, openrouter.WithToken(cfg.Token))
	}

	if model.Client != nil {
		options = append(options, openrouter.WithClient(model.Client))
	}

	return openrouter.NewSynthesizer(model.ID, options...)
}
