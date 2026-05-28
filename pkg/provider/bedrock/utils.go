package bedrock

import (
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

func isAdaptiveThinkingModel(model string) bool {
	model = strings.ToLower(model)

	thinkingPatterns := []string{
		"sonnet-4-6",

		"opus-4-8",
		"opus-4-7",
		"opus-4-6",
	}

	for _, p := range thinkingPatterns {
		if strings.Contains(model, p) {
			return true
		}
	}

	return false
}

func adaptiveEffort(e provider.Effort) (effort string, enabled bool) {
	switch e {
	case provider.EffortAdaptive:
		return "", true
	case provider.EffortMinimal, provider.EffortLow:
		return "low", true
	case provider.EffortMedium:
		return "medium", true
	case provider.EffortHigh:
		return "high", true
	case provider.EffortXHigh:
		return "xhigh", true
	case provider.EffortMax:
		return "max", true
	}
	return "", false
}
