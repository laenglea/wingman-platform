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

func outputEffort(e provider.Effort) string {
	switch e {
	case provider.EffortMinimal, provider.EffortLow:
		return "low"
	case provider.EffortMedium:
		return "medium"
	case provider.EffortHigh:
		return "high"
	case provider.EffortXHigh:
		return "xhigh"
	case provider.EffortMax:
		return "max"
	}
	return ""
}
