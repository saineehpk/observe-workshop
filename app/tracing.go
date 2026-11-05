package main

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer = otel.Tracer("observe-workshop")

// setupTracing creates real spans and sends them to Tempo over OTLP/HTTP
// (OTEL_EXPORTER_OTLP_ENDPOINT). Trace context crosses services in the W3C
// `traceparent` header. In legacy mode spans are still created, so trace ids
// exist, but nothing is exported: "before observe" has no traces to look at.
func setupTracing(service string, export bool) func(context.Context) error {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", service),
			attribute.String("service.namespace", "workshop"),
		)),
	}
	if export && os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, err := otlptracehttp.New(context.Background())
		if err != nil {
			log.Fatalf("otlp exporter: %v", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tracer = tp.Tracer("observe-workshop")
	return tp.Shutdown
}

func traceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

func spanID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}
