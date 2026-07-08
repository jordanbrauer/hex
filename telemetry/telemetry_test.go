package telemetry_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/jordanbrauer/hex/telemetry"
)

func TestSetup_requiresServiceName(t *testing.T) {
	_, err := telemetry.Setup(context.Background(), telemetry.Options{})
	if err == nil {
		t.Errorf("empty ServiceName returned nil error")
	}
}

func TestSetup_installsGlobalProviders(t *testing.T) {
	p, err := telemetry.Setup(context.Background(), telemetry.Options{
		ServiceName:    "hex-test",
		ServiceVersion: "0.0.0",
		Environment:    "test",
		Exporter:       telemetry.ExporterNone,
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	defer p.Shutdown(context.Background())

	// The global providers should now be ours.
	if otel.GetTracerProvider() == nil {
		t.Errorf("global TracerProvider not set")
	}

	if otel.GetMeterProvider() == nil {
		t.Errorf("global MeterProvider not set")
	}

	// Tracer helper delegates to the global.
	tracer := telemetry.Tracer("hex/test")
	if tracer == nil {
		t.Errorf("Tracer() returned nil")
	}

	// Same for Meter.
	meter := telemetry.Meter("hex/test")
	if meter == nil {
		t.Errorf("Meter() returned nil")
	}
}

func TestSetup_spanEmission(t *testing.T) {
	p, err := telemetry.Setup(context.Background(), telemetry.Options{
		ServiceName: "hex-test",
		Exporter:    telemetry.ExporterNone,
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	defer p.Shutdown(context.Background())

	tracer := telemetry.Tracer("hex/telemetry_test")

	ctx, span := tracer.Start(context.Background(), "unit-of-work")
	span.End()

	// Trace ID should be valid inside the span context.
	if !span.SpanContext().IsValid() {
		t.Errorf("span context invalid")
	}

	// A child span shares the same trace.
	_, child := tracer.Start(ctx, "child")

	if child.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Errorf("child span TraceID differs from parent")
	}

	child.End()
}

func TestSetup_resourceCarriesAttributes(t *testing.T) {
	p, err := telemetry.Setup(context.Background(), telemetry.Options{
		ServiceName:    "hex-test",
		ServiceVersion: "1.2.3",
		Environment:    "staging",
		ResourceAttributes: map[string]string{
			"team": "platform",
		},
		Exporter: telemetry.ExporterNone,
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	defer p.Shutdown(context.Background())

	res := p.Resource()
	if res == nil {
		t.Fatal("Resource returned nil")
	}

	attrs := map[string]string{}
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}

	if attrs["service.name"] != "hex-test" {
		t.Errorf("service.name = %q", attrs["service.name"])
	}

	if attrs["service.version"] != "1.2.3" {
		t.Errorf("service.version = %q", attrs["service.version"])
	}

	if attrs["deployment.environment"] != "staging" {
		t.Errorf("deployment.environment = %q", attrs["deployment.environment"])
	}

	if attrs["team"] != "platform" {
		t.Errorf("team = %q", attrs["team"])
	}
}

func TestShutdown_idempotent(t *testing.T) {
	p, err := telemetry.Setup(context.Background(), telemetry.Options{
		ServiceName: "hex-test",
		Exporter:    telemetry.ExporterNone,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("first Shutdown: %v", err)
	}

	// OTel's TracerProvider.Shutdown is idempotent per spec — call again.
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("second Shutdown: %v", err)
	}
}

func TestProviders_exposedForAdvancedUse(t *testing.T) {
	p, _ := telemetry.Setup(context.Background(), telemetry.Options{
		ServiceName: "hex-test",
		Exporter:    telemetry.ExporterNone,
	})

	defer p.Shutdown(context.Background())

	if p.TracerProvider() == nil {
		t.Errorf("TracerProvider() nil")
	}

	if p.MeterProvider() == nil {
		t.Errorf("MeterProvider() nil")
	}
}
