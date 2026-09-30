package otel

import (
	"context"
	"time"

	"github.com/adrianliechti/wingman/pkg/provider"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/semconv/v1.41.0/genaiconv"
	"go.opentelemetry.io/otel/trace"
)

type Decider interface {
	Observable
	provider.Decider
}

type observableDecider struct {
	model    string
	provider string
	decider  provider.Decider

	tokenUsageMetric        genaiconv.ClientTokenUsage
	operationDurationMetric genaiconv.ClientOperationDuration
}

func NewDecider(provider, model string, p provider.Decider) Decider {
	meter := otel.Meter(instrumentationName)
	tokens, _ := genaiconv.NewClientTokenUsage(meter)
	duration, _ := genaiconv.NewClientOperationDuration(meter)
	return &observableDecider{
		model: model, provider: provider, decider: p,
		tokenUsageMetric: tokens, operationDurationMetric: duration,
	}
}

func (p *observableDecider) otelSetup() {}

func (p *observableDecider) Decide(ctx context.Context, input *provider.DecisionInput) (*provider.Decision, error) {
	operation := genaiconv.OperationNameAttr("decisions")
	ctx, span := otel.Tracer(instrumentationName).Start(ctx, GenAISpanName(operation, p.model), trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(RequestAttrs(semconv.GenAIOperationNameKey.String(string(operation)), p.provider, p.model)...)

	start := time.Now()
	result, err := p.decider.Decide(ctx, input)
	if err != nil {
		RecordError(span, err)
	}
	responseModel := p.model
	if result != nil {
		if result.Model != "" {
			responseModel = result.Model
		}
		span.SetAttributes(KeyValues(
			[]KeyValue{semconv.GenAIResponseModel(responseModel), semconv.GenAIResponseID(result.ID)},
			UsageAttrs(result.Usage),
		)...)
		if result.Usage != nil {
			attrs := MetricAttrs(ctx, p.model, responseModel)
			if result.Usage.InputTokens > 0 {
				p.tokenUsageMetric.Record(ctx, int64(result.Usage.InputTokens), operation,
					genaiconv.ProviderNameAttr(p.provider), genaiconv.TokenTypeInput, attrs...)
			}
			if result.Usage.OutputTokens > 0 {
				p.tokenUsageMetric.Record(ctx, int64(result.Usage.OutputTokens), operation,
					genaiconv.ProviderNameAttr(p.provider), genaiconv.TokenTypeOutput, attrs...)
			}
		}
	}
	attrs := MetricAttrs(ctx, p.model, responseModel)
	if err != nil {
		attrs = append(attrs, p.operationDurationMetric.AttrErrorType(ErrorTypeAttr(err)))
	}
	p.operationDurationMetric.Record(ctx, time.Since(start).Seconds(), operation, genaiconv.ProviderNameAttr(p.provider), attrs...)
	return result, err
}
