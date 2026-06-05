package provider

import "testing"

func TestToolFlattenRoundTrip(t *testing.T) {
	tools := []Tool{
		{Name: "plain_tool"},
		{Name: "mcp__weather__", Tools: []Tool{
			{Name: "get_weather"},
			{Name: "get_forecast"},
		}},
	}

	flat := FlattenTools(tools)
	if len(flat) != 3 {
		t.Fatalf("expected 3 flattened tools, got %d: %+v", len(flat), flat)
	}
	if flat[1].Name != "mcp__weather___get_weather" || flat[1].Namespace != "mcp__weather__" {
		t.Fatalf("unexpected flattened tool: %+v", flat[1])
	}

	aliases := ToolAliases(tools)
	if len(aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d: %+v", len(aliases), aliases)
	}

	call := UnflattenToolCall(aliases, ToolCall{ID: "call_1", Name: "mcp__weather___get_weather"})
	if call.Name != "get_weather" || call.Namespace != "mcp__weather__" {
		t.Fatalf("unexpected unflattened call: %+v", call)
	}

	if name := FlattenToolName(call); name != "mcp__weather___get_weather" {
		t.Fatalf("round trip mismatch: got %q", name)
	}

	plain := UnflattenToolCall(aliases, ToolCall{ID: "call_2", Name: "plain_tool"})
	if plain.Name != "plain_tool" || plain.Namespace != "" {
		t.Fatalf("plain call should pass through: %+v", plain)
	}

	if name := FlattenToolName(plain); name != "plain_tool" {
		t.Fatalf("plain name should pass through: got %q", name)
	}
}
