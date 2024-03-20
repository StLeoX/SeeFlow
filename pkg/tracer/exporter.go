package tracer

import (
	"context"
	"fmt"
	attr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktr "go.opentelemetry.io/otel/sdk/trace"
)

func (tm *TracerManager) InitGRPCExporter(shutdownCtx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(shutdownCtx)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC exporter: %w", err)
	}

	tm.tracerProvider = sdktr.NewTracerProvider(
		sdktr.WithBatcher(exporter),
		sdktr.WithResource(resource.Empty()))

	return tm.tracerProvider.Shutdown, nil

}

func (tm *TracerManager) InitStdoutExporter() (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("creating stdout exporter: %w", err)
	}

	tm.tracerProvider = sdktr.NewTracerProvider(
		sdktr.WithBatcher(exporter),
		sdktr.WithResource(resource.Empty()))

	return tm.tracerProvider.Shutdown, nil
}

// InitDummyExporter only for testing purposes
func (tm *TracerManager) InitDummyExporter() (func(context.Context) error, error) {
	tm.tracerProvider = sdktr.NewTracerProvider(
		sdktr.WithResource(resource.NewSchemaless(attr.Bool("debug", true))),
	)
	return tm.tracerProvider.Shutdown, nil
}
