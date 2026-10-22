package anthropic

import "fmt"

func validateAPIVersion(version string, messages []MessageParam) error {
	switch version {
	case "", "2023-06-01", "2026-10-21":
	default:
		return fmt.Errorf("unsupported anthropic-version %q", version)
	}
	for i, message := range messages {
		blocks, _ := message.Content.([]any)
		for j, raw := range blocks {
			block, _ := raw.(map[string]any)
			path := fmt.Sprintf("messages[%d].content[%d]", i, j)
			if version == "2026-10-21" && message.Role == MessageRoleAssistant && block["type"] == "document" {
				return fmt.Errorf("%s: document blocks must be in user messages", path)
			}
			if err := validateToolCitations(block, path, false); err != nil {
				return err
			}
			if version == "2026-10-21" && message.Role == MessageRoleUser {
				if err := validateUserCitations(block, path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateUserCitations(block map[string]any, path string) error {
	if block["type"] == "text" {
		if citations, _ := block["citations"].([]any); len(citations) > 0 {
			return fmt.Errorf("%s.citations: citations are not supported in user messages", path)
		}
	}
	if block["type"] == "search_result" || block["type"] == "tool_result" {
		content, _ := block["content"].([]any)
		for i, raw := range content {
			part, _ := raw.(map[string]any)
			if err := validateUserCitations(part, fmt.Sprintf("%s.content[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateToolCitations(block map[string]any, path string, toolResult bool) error {
	if toolResult && block["type"] == "text" {
		if citations, _ := block["citations"].([]any); len(citations) > 0 {
			return fmt.Errorf("%s.citations: citations are not supported in tool result text", path)
		}
	}
	content, _ := block["content"].([]any)
	for i, raw := range content {
		part, _ := raw.(map[string]any)
		if err := validateToolCitations(part, fmt.Sprintf("%s.content[%d]", path, i), block["type"] == "tool_result"); err != nil {
			return err
		}
	}
	return nil
}
