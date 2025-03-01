package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	Meter metric.Meter
)

// InitTelemetry initializes OpenTelemetry with the OTLP exporter
func InitTelemetry(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
	if !cfg.Telemetry.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Telemetry.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configure OTLP exporter with timeout
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	collectorAddr := fmt.Sprintf("%s:%d", cfg.Telemetry.OTELCollector.Host, cfg.Telemetry.OTELCollector.Port)
	conn, err := grpc.DialContext(dialCtx, collectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Remove blocking dial
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		// Log warning but continue without telemetry
		fmt.Printf("Warning: Failed to connect to OpenTelemetry collector: %v\n", err)
		fmt.Printf("The application will continue without telemetry\n")
		return func(context.Context) error { return nil }, nil
	}

	// Set up trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		fmt.Printf("Warning: Failed to create trace exporter: %v\n", err)
		conn.Close()
		return func(context.Context) error { return nil }, nil
	}

	// Set up trace provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Set up metrics exporter
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		fmt.Printf("Warning: Failed to create metric exporter: %v\n", err)
		conn.Close()
		return func(context.Context) error { return nil }, nil
	}

	// Set up metric provider with configured interval
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(cfg.Telemetry.Metrics.Interval),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	// Create a meter
	Meter = meterProvider.Meter(cfg.Telemetry.ServiceName)

	// Return cleanup function
	return func(ctx context.Context) error {
		cctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		var errs []error
		if err := tracerProvider.Shutdown(cctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
		if err := meterProvider.Shutdown(cctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close gRPC connection: %w", err))
		}

		if len(errs) > 0 {
			return fmt.Errorf("shutdown errors: %v", errs)
		}
		return nil
	}, nil
}
