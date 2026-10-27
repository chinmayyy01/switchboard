package obs

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const service = "switchboard"

func Init(ctx context.Context) (func(context.Context) error, error) {
	mode := os.Getenv("OTEL_TRACES")
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if mode != "stdout" && endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	var exp sdktrace.SpanExporter
	var err error
	if mode == "stdout" {
		exp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	} else {
		exp, err = otlptracehttp.New(ctx)
	}
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", service)))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	return tp.Shutdown, nil
}

func Tracer() trace.Tracer { return otel.Tracer(service) }
