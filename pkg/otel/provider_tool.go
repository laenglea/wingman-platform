package otel

import (
	"context"
	"iter"

	"github.com/adrianliechti/wingman/pkg/provider"
	"github.com/adrianliechti/wingman/pkg/tool"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/semconv/v1.41.0"
)

type Tool interface {
	Observable
	tool.Provider
}

type observableTool struct {
	provider string

	tool tool.Provider
}

func NewTool(provider string, p tool.Provider) Tool {
	return &observableTool{
		tool: p,

		provider: provider,
	}
}

func (p *observableTool) otelSetup() {
}

func (p *observableTool) Tools(ctx context.Context) ([]tool.Tool, error) {
	ctx, span := otel.Tracer(instrumentationName).Start(ctx, "tools")
	defer span.End()

	tools, err := p.tool.Tools(ctx)

	return tools, err
}

func (p *observableTool) Execute(ctx context.Context, name string, parameters map[string]any) (any, error) {
	return tool.Collect(p.ExecuteStream(ctx, name, parameters), nil)
}

func (p *observableTool) ExecuteStream(ctx context.Context, name string, parameters map[string]any) iter.Seq2[*tool.Execution, error] {
	return func(yield func(*tool.Execution, error) bool) {
		ctx, span := otel.Tracer(instrumentationName).Start(ctx, "execute_tool "+name)
		defer span.End()

		if span.IsRecording() {
			baseAttrs := []KeyValue{
				semconv.GenAIOperationNameExecuteTool,
				semconv.GenAIToolName(name),
				semconv.GenAIToolType("function"),
			}

			span.SetAttributes(KeyValues(
				baseAttrs,
				ToolArgumentAttrs(parameters),
			)...)
		}

		for execution, err := range tool.ExecuteStream(ctx, p.tool, name, parameters) {
			if err != nil {
				RecordError(span, err)
				yield(nil, err)
				return
			}
			if execution != nil && execution.Progress == "" && span.IsRecording() {
				span.SetAttributes(ToolResultAttrs(execution.Result)...)
			}
			if !yield(execution, nil) {
				return
			}
		}
	}
}

func (p *observableTool) Result(name string, value any) provider.ToolResult {
	result, err := tool.RenderResult(p.tool, name, value)
	if err != nil {
		result = tool.TextResult("Error: " + err.Error())
		result.IsError = true
	}
	return result
}
