package config

import (
	"errors"
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/provider/azurespeech"
	"github.com/adrianliechti/wingman/pkg/provider/openai"
	"github.com/adrianliechti/wingman/pkg/provider/openrouter"
)

func (cfg *Config) RegisterTranscriber(id string, p provider.Transcriber) {
	cfg.RegisterModel(id)

	if cfg.transcriber == nil {
		cfg.transcriber = make(map[string]provider.Transcriber)
	}

	if _, ok := cfg.transcriber[""]; !ok {
		cfg.transcriber[""] = p
	}

	cfg.transcriber[id] = p
}

func (cfg *Config) Transcriber(id string) (provider.Transcriber, error) {
	if cfg.transcriber != nil {
		if t, ok := cfg.transcriber[id]; ok {
			return t, nil
		}
	}

	return nil, errors.New("transcriber not found: " + id)
}

func createTranscriber(cfg providerConfig, model modelContext) (provider.Transcriber, error) {
	switch strings.ToLower(cfg.Type) {
	case "mistral":
		if cfg.URL == "" {
			cfg.URL = "https://api.mistral.ai/v1/"
		}

		return openaiTranscriber(cfg, model)

	case "openai", "openai-compatible":
		return openaiTranscriber(cfg, model)

	case "openrouter":
		return openrouterTranscriber(cfg, model)

	case "azurespeech", "azure-speech":
		return azureSpeechTranscriber(cfg, model)

	default:
		return nil, errors.New("invalid transcriber type: " + cfg.Type)
	}
}

func azureSpeechTranscriber(cfg providerConfig, model modelContext) (provider.Transcriber, error) {
	var options []azurespeech.Option

	if cfg.Token != "" {
		options = append(options, azurespeech.WithToken(cfg.Token))
	}

	if model.Client != nil {
		options = append(options, azurespeech.WithClient(model.Client))
	}

	region := cfg.Vars["region"]

	return azurespeech.NewTranscriber(region, model.ID, options...)
}

func openaiTranscriber(cfg providerConfig, model modelContext) (provider.Transcriber, error) {
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

	return openai.NewTranscriber(cfg.URL, model.ID, options...)
}

func openrouterTranscriber(cfg providerConfig, model modelContext) (provider.Transcriber, error) {
	var options []openrouter.Option

	if cfg.Token != "" {
		options = append(options, openrouter.WithToken(cfg.Token))
	}

	if model.Client != nil {
		options = append(options, openrouter.WithClient(model.Client))
	}

	return openrouter.NewTranscriber(model.ID, options...)
}
