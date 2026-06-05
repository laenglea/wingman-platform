package provider

func FlattenTools(tools []Tool) []Tool {
	if !hasNested(tools) {
		return tools
	}

	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		if len(t.Tools) == 0 {
			out = append(out, t)
			continue
		}
		for _, child := range t.Tools {
			flat := child
			flat.Name = flattenedToolName(t.Name, child.Name)
			flat.Namespace = t.Name
			flat.Tools = nil
			out = append(out, flat)
		}
	}
	return out
}

func ToolAliases(tools []Tool) map[string]Tool {
	result := map[string]Tool{}

	for _, t := range tools {
		for _, child := range t.Tools {
			result[flattenedToolName(t.Name, child.Name)] = Tool{Name: child.Name, Namespace: t.Name}
		}
	}

	return result
}

func UnflattenToolCall(aliases map[string]Tool, call ToolCall) ToolCall {
	if alias, ok := aliases[call.Name]; ok {
		call.Name = alias.Name
		call.Namespace = alias.Namespace
	}

	return call
}

func FlattenToolName(call ToolCall) string {
	if call.Namespace != "" && call.Name != "" {
		return flattenedToolName(call.Namespace, call.Name)
	}

	return call.Name
}

func flattenedToolName(namespace, name string) string {
	return namespace + "_" + name
}

func hasNested(tools []Tool) bool {
	for _, t := range tools {
		if len(t.Tools) > 0 {
			return true
		}
	}
	return false
}
