package bedrock

import "strings"

func isAdaptiveThinkingModel(model string) bool {
	model = strings.ToLower(model)

	thinkingPatterns := []string{
		"sonnet-4-6",
		"opus-4-6",
	}

	for _, p := range thinkingPatterns {
		if strings.Contains(model, p) {
			return true
		}
	}

	return false
}
