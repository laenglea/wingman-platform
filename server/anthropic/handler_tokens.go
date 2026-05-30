package anthropic

import (
	"encoding/json"
	"net/http"
)

const (
	charsPerToken     = 4
	jsonCharsPerToken = 3

	// Structural framing per message (role delimiters and separators).
	messageTokenOverhead = 3

	// System prompt the API injects whenever any tool is defined.
	toolUseSystemOverhead = 264

	// Flat per-image estimate; Anthropic downsizes to ~1.15MP before counting.
	imageTokens = 1300
)

func (h *Handler) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	var req CountTokensRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var tokens int

	if req.System != nil {
		system, err := parseSystemContent(req.System)

		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		tokens += textTokens(system)
	}

	for _, msg := range req.Messages {
		tokens += messageTokens(msg)
	}

	if len(req.Tools) > 0 {
		tokens += toolUseSystemOverhead

		for _, tool := range req.Tools {
			tokens += toolTokens(tool)
		}
	}

	writeJson(w, CountTokensResponse{
		InputTokens: tokens,
	})
}

func messageTokens(msg MessageParam) int {
	blocks, err := parseContentBlocks(msg.Content)
	if err != nil {
		return messageTokenOverhead + jsonTokens(msg.Content)
	}

	return messageTokenOverhead + blocksTokens(blocks)
}

func blocksTokens(blocks []ContentBlockParam) int {
	var total int

	for _, block := range blocks {
		switch block.Type {
		case "text":
			total += textTokens(block.Text)
		case "thinking":
			total += textTokens(block.Thinking)
		case "tool_use":
			total += textTokens(block.Name)
			total += jsonTokens(block.Input)
		case "tool_result":
			total += toolResultTokens(block.Content)
		case "image":
			total += imageTokens
		}
	}

	return total
}

func toolResultTokens(content any) int {
	blocks, err := parseContentBlocks(content)
	if err != nil {
		return jsonTokens(content)
	}

	return blocksTokens(blocks)
}

func toolTokens(tool ToolParam) int {
	total := textTokens(tool.Name)
	total += textTokens(tool.Description)
	total += jsonTokens(tool.InputSchema)

	return total
}

func textTokens(s string) int {
	return len(s) / charsPerToken
}

func jsonTokens(v any) int {
	data, err := json.Marshal(v)
	if err != nil {
		return 0
	}

	return len(data) / jsonCharsPerToken
}
