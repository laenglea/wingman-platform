package mcp

type MCP struct {
	Object string `json:"object"` // "mcp"

	ID string `json:"id"`
}

type MCPList struct {
	Object string `json:"object"` // "list"

	MCPs []MCP `json:"data"`
}
