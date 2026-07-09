// Package telemetry sets up OpenTelemetry tracing and metrics for hex
// applications.
//
// One Setup call wires:
//
//   - a tracer provider with the configured exporter,
//   - a meter provider with the configured exporter,
//   - a resource carrying service name/version,
//   - propagation (tracecontext + baggage),
//   - global registration so any library reading `otel.Tracer(...)` sees them.
//
// The returned Provider carries a single Shutdown method that flushes and
// closes every configured exporter, ready to defer at the end of main.
//
// See ADR-0014.
//
// Example:
//
//	tp, err := telemetry.Setup(ctx, telemetry.Options{
//	    ServiceName:    "myapp",
//	    ServiceVersion: build.Version(),
//	    Exporter:       telemetry.ExporterStdout,
//	})
//	if err != nil { return err }
//	defer tp.Shutdown(context.Background())
//
//	tracer := telemetry.Tracer("myapp/http")
//	ctx, span := tracer.Start(ctx, "handle-request")
//	defer span.End()
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Exporter selects the exporter backend.
type Exporter int

const (
	// ExporterNone disables the exporter entirely — spans/metrics are
	// discarded. Useful in tests.
	ExporterNone Exporter = iota
	// ExporterStdout writes spans and metrics as JSON to stdout.
	// Default for dev.
	ExporterStdout
	// ExporterOTLP ships to an OTLP-compatible collector over gRPC.
	// Production default. Endpoint is picked from OTEL_EXPORTER_OTLP_ENDPOINT
	// environment variable per the OTel spec.
	ExporterOTLP
)

// Options configures Setup.
type Options struct {
	// ServiceName identifies the service in traces and metrics. Required.
	ServiceName string

	// ServiceVersion is optional but strongly recommended for correlating
	// telemetry to specific builds. Feed hex/build.Version() here.
	ServiceVersion string

	// Environment tags telemetry with the deployment environment (e.g.
	// "dev", "staging", "production"). Optional.
	Environment string

	// Exporter selects the exporter. Defaults to ExporterStdout.
	Exporter Exporter

	// ResourceAttributes are additional key/value pairs attached to every
	// emitted span and metric.
	ResourceAttributes map[string]string

	// TraceExporter, MetricExporter override the exporter for one signal.
	// Useful when a consumer wants OTLP for traces and stdout for metrics
	// (or a custom exporter neither shipped here supports).
	TraceExporter  sdktrace.SpanExporter
	MetricExporter sdkmetric.Exporter
}

// Provider carries the tracer and meter providers plus a Shutdown method
// that flushes and closes them.
type Provider struct {
	tracer   *sdktrace.TracerProvider
	meter    *sdkmetric.MeterProvider
	res      *resource.Resource
	shutdown atomic.Bool
}

// Setup configures OTel with opts and returns a Provider whose Shutdown
// method should be deferred at the end of main.
func Setup(ctx context.Context, opts Options) (*Provider, error) {
	if opts.ServiceName == "" {
		return nil, errors.New("telemetry: ServiceName is required")
	}

	res, err := buildResource(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	traceExp, err := chooseTraceExporter(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}

	metricExp, err := chooseMetricExporter(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}

	var (
		tp *sdktrace.TracerProvider
		mp *sdkmetric.MeterProvider
	)

	if traceExp != nil {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(res),
		)
	} else {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}

	if metricExp != nil {
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
			sdkmetric.WithResource(res),
		)
	} else {
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	}

	// Install globals so any library reading otel.Tracer / otel.Meter
	// sees our providers.
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{tracer: tp, meter: mp, res: res}, nil
}

// Shutdown flushes and closes the tracer and meter providers. Idempotent —
// subsequent calls after the first are no-ops.
func (p *Provider) Shutdown(ctx context.Context) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error

	if err := p.tracer.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("tracer: %w", err))
	}

	if err := p.meter.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("meter: %w", err))
	}

	return errors.Join(errs...)
}

// TracerProvider returns the underlying SDK tracer provider. Escape hatch
// for consumers that need to add SpanProcessor, override sampling, etc.
func (p *Provider) TracerProvider() *sdktrace.TracerProvider { return p.tracer }

// MeterProvider returns the underlying SDK meter provider.
func (p *Provider) MeterProvider() *sdkmetric.MeterProvider { return p.meter }

// Resource returns the attached resource. Handy for asserting attributes
// in tests.
func (p *Provider) Resource() *resource.Resource { return p.res }

// -- Package-level shortcuts (delegating to otel globals) ----------------

// Tracer returns a Tracer with the given instrumentation name. Backed by
// the global TracerProvider Setup installed.
func Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return otel.Tracer(name, opts...)
}

// Meter returns a Meter with the given instrumentation name.
func Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return otel.Meter(name, opts...)
}

// -- exporter selection ---------------------------------------------------

func chooseTraceExporter(ctx context.Context, opts Options) (sdktrace.SpanExporter, error) {
	if opts.TraceExporter != nil {
		return opts.TraceExporter, nil
	}

	switch opts.Exporter {
	case ExporterNone:
		return nil, nil
	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithWriter(os.Stdout), stdouttrace.WithPrettyPrint())
	case ExporterOTLP:
		return otlptracegrpc.New(ctx)
	default:
		return stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	}
}

func chooseMetricExporter(ctx context.Context, opts Options) (sdkmetric.Exporter, error) {
	if opts.MetricExporter != nil {
		return opts.MetricExporter, nil
	}

	switch opts.Exporter {
	case ExporterNone:
		return nil, nil
	case ExporterStdout:
		return stdoutmetric.New()
	case ExporterOTLP:
		return otlpmetricgrpc.New(ctx)
	default:
		return stdoutmetric.New()
	}
}

func buildResource(ctx context.Context, opts Options) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(semconv.ServiceName(opts.ServiceName)),
	}

	if opts.ServiceVersion != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.ServiceVersion(opts.ServiceVersion)))
	}

	if opts.Environment != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.DeploymentEnvironment(opts.Environment)))
	}

	for k, v := range opts.ResourceAttributes {
		attrs = append(attrs, resource.WithAttributes(attribute.String(k, v)))
	}

	return resource.New(ctx, attrs...)
}
