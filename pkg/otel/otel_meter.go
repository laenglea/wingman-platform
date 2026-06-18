package otel

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
)

func setupMeter(ctx context.Context, resource *sdkresource.Resource) error {
	var err error
	var exporter sdkmetric.Exporter

	if strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")) == "grpc" || strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")) == "grpc" {
		exporter, err = otlpmetricgrpc.New(ctx)
	} else {
		exporter, err = otlpmetrichttp.New(ctx)
	}

	if err != nil {
		return err
	}

	options := []sdkmetric.Option{
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(30*time.Second))),
	}

	if endpoint := os.Getenv("INSIGHTS_ENDPOINT"); endpoint != "" {
		if insights, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint)); err == nil {
			options = append(options, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(insights, sdkmetric.WithInterval(60*time.Second))))
		}
	}

	provider := sdkmetric.NewMeterProvider(options...)

	otel.SetMeterProvider(provider)

	return nil
}
