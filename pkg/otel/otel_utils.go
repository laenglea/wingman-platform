package otel

import (
	"context"

	"github.com/adrianliechti/wingman/pkg/auth"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/semconv/v1.40.0/genaiconv"
)

// Cache token type attributes following the GenAI semantic conventions:
// gen_ai.usage.cache_creation.input_tokens and gen_ai.usage.cache_read.input_tokens
var (
	TokenTypeCacheCreation genaiconv.TokenTypeAttr = "cache_creation"
	TokenTypeCacheRead     genaiconv.TokenTypeAttr = "cache_read"
)

type KeyValue = attribute.KeyValue

func String(key string, val string) KeyValue {
	return attribute.String(key, val)
}

func Strings(key string, val []string) KeyValue {
	return attribute.StringSlice(key, val)
}

func KeyValues(attrs ...[]KeyValue) []KeyValue {
	var result []KeyValue

	for _, a := range attrs {
		result = append(result, a...)
	}

	return result
}

func Label(ctx context.Context, attrs ...KeyValue) {
	labeler, ok := otelhttp.LabelerFromContext(ctx)

	if !ok {
		return
	}

	labeler.Add(attrs...)
}

func EndUserAttrs(ctx context.Context) []KeyValue {
	var attrs []KeyValue

	if user, ok := ctx.Value(auth.UserContextKey).(string); ok && user != "" {
		attrs = append(attrs,
			attribute.String("user.id", user),
			attribute.String("enduser.id", user), // deprecated
		)
	}

	if email, ok := ctx.Value(auth.EmailContextKey).(string); ok && email != "" {
		attrs = append(attrs,
			attribute.String("user.email", email),
			attribute.String("enduser.email", email), // deprecated
		)
	}

	if name, ok := ctx.Value(auth.NameContextKey).(string); ok && name != "" {
		attrs = append(attrs,
			attribute.String("user.full_name", name),
			attribute.String("enduser.name", name), // deprecated
		)
	}

	return attrs
}
