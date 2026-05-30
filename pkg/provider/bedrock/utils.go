package bedrock

import (
	"strings"

	"github.com/adrianliechti/wingman/pkg/provider"
)

var LegacyModels = []string{
	"claude-3",

	"sonnet-4-0",
	"sonnet-4-5",

	"opus-4-0",
	"opus-4-1",
	"opus-4-5",

	"haiku-4-5",
}

func isLegacyModel(model string) bool {
	model = strings.ToLower(model)

	for _, p := range LegacyModels {
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
