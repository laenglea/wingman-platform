package router

import "github.com/adrianliechti/wingman/pkg/provider"

// ScrubMessages returns a copy of messages with provider-signed Reasoning
// and Compaction blocks removed. These blocks carry HMAC/AEAD signatures
// keyed per-provider and per-tenant, so they cannot be replayed against a
// different provider in a stateless router.
func ScrubMessages(messages []provider.Message) []provider.Message {
	result := make([]provider.Message, len(messages))

	for i, m := range messages {
		result[i] = provider.Message{
			Role:    m.Role,
			Content: scrubContent(m.Content),
		}
	}

	return result
}

func scrubContent(content []provider.Content) []provider.Content {
	result := make([]provider.Content, 0, len(content))

	for _, c := range content {
		if c.Reasoning != nil || c.Compaction != nil {
			continue
		}

		result = append(result, c)
	}

	return result
}

// ScrubOptions returns a copy of options with signature requests disabled.
// Signatures issued by one provider cannot be validated by another, so a
// router fronting multiple providers should never request them.
func ScrubOptions(options *provider.CompleteOptions) *provider.CompleteOptions {
	if options == nil || options.ReasoningOptions == nil {
		return options
	}

	if !options.ReasoningOptions.IncludeSignature {
		return options
	}

	cloned := *options
	reasoning := *options.ReasoningOptions
	reasoning.IncludeSignature = false
	cloned.ReasoningOptions = &reasoning

	return &cloned
}
