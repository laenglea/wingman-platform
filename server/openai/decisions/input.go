package decisions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func validateInput(raw json.RawMessage) error {
	if absent(raw) {
		return errors.New("input is required")
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return nil
	}
	var items []struct {
		Role      string          `json:"role"`
		Type      string          `json:"type"`
		Content   json.RawMessage `json:"content"`
		CallID    string          `json:"call_id"`
		Name      string          `json:"name"`
		Arguments *string         `json:"arguments"`
		Input     *string         `json:"input"`
		Output    json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return errors.New("input must be text or an array of messages and tool items")
	}
	calls := map[string]string{}
	for i, item := range items {
		switch item.Type {
		case "", "message":
			switch item.Role {
			case "user", "assistant", "developer", "system":
			default:
				return fmt.Errorf("input[%d]: unsupported role %q", i, item.Role)
			}
			if err := validateParts(item.Content, item.Role); err != nil {
				return fmt.Errorf("input[%d].content: %w", i, err)
			}
		case "function_call", "custom_tool_call":
			if item.CallID == "" || item.Name == "" || calls[item.CallID] != "" {
				return fmt.Errorf("input[%d]: tool call requires a unique call_id and name", i)
			}
			if item.Type == "function_call" && item.Arguments == nil || item.Type == "custom_tool_call" && item.Input == nil {
				return fmt.Errorf("input[%d]: tool call arguments or input are required", i)
			}
			calls[item.CallID] = item.Type
		case "function_call_output", "custom_tool_call_output":
			if calls[item.CallID] != strings.TrimSuffix(item.Type, "_output") {
				return fmt.Errorf("input[%d]: tool output requires a matching earlier call", i)
			}
			if err := validateParts(item.Output, "tool"); err != nil {
				return fmt.Errorf("input[%d].output: %w", i, err)
			}
		default:
			return fmt.Errorf("input[%d]: unsupported type %q", i, item.Type)
		}
	}
	return nil
}

func absent(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validateParts(raw json.RawMessage, role string) error {
	if absent(raw) {
		return errors.New("content is required")
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return nil
	}
	var parts []struct {
		Type       string          `json:"type"`
		Text       *string         `json:"text"`
		Refusal    *string         `json:"refusal"`
		Data       json.RawMessage `json:"data"`
		Breakpoint json.RawMessage `json:"prompt_cache_breakpoint"`
	}
	if json.Unmarshal(raw, &parts) != nil || len(parts) == 0 {
		return errors.New("content must be text or a nonempty array of parts")
	}
	for _, part := range parts {
		if !absent(part.Breakpoint) {
			return errors.New("cache breakpoints are not supported in decisions")
		}
		switch part.Type {
		case "input_text":
			if role == "assistant" || part.Text == nil {
				return errors.New("input_text requires text and a non-assistant role")
			}
		case "output_text":
			if role != "assistant" || part.Text == nil {
				return errors.New("output_text requires text and an assistant role")
			}
		case "refusal":
			if role != "assistant" || part.Refusal == nil {
				return errors.New("refusal requires refusal text and an assistant role")
			}
		case "input_json":
			data := bytes.TrimSpace(part.Data)
			if role != "user" && role != "question" || len(data) == 0 || data[0] != '{' && data[0] != '[' {
				return errors.New("input_json requires object or array data in a user message or question")
			}
		case "input_image":
			return errors.New("image inputs are not supported by decisions; use a multimodal completion")
		default:
			return fmt.Errorf("unsupported content type %q", part.Type)
		}
	}
	return nil
}
