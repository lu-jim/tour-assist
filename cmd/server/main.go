package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat"
	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/httpx"
	"github.com/acai-travel/tech-challenge/internal/mongox"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// initResource creates a resource with service information
func initResource() (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("acai-chat-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
}

// initMeterProvider initializes an OpenTelemetry MeterProvider with a stdout exporter
func initMeterProvider() (*metric.MeterProvider, error) {
	// Create stdout exporter
	exporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := initResource()
	if err != nil {
		return nil, err
	}

	// Create a meter provider with a periodic reader
	// The reader will export metrics every 10 seconds
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter,
			metric.WithInterval(10*time.Second))),
		metric.WithResource(res),
	)

	// Set the global meter provider
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

// initTracerProvider initializes an OpenTelemetry TracerProvider with a stdout exporter
func initTracerProvider() (*sdktrace.TracerProvider, error) {
	// Create stdout exporter for traces
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := initResource()
	if err != nil {
		return nil, err
	}

	// Create a tracer provider with a batch span processor
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set the global propagator to use W3C Trace Context
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider, nil
}

func main() {
	// Initialize OpenTelemetry meter provider
	meterProvider, err := initMeterProvider()
	if err != nil {
		slog.Error("Failed to initialize meter provider", "error", err)
		panic(err)
	}

	// Initialize OpenTelemetry tracer provider
	tracerProvider, err := initTracerProvider()
	if err != nil {
		slog.Error("Failed to initialize tracer provider", "error", err)
		panic(err)
	}

	// Setup graceful shutdown for metrics and traces
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := meterProvider.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown meter provider", "error", err)
		}
		if err := tracerProvider.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown tracer provider", "error", err)
		}
	}()

	mongo := mongox.MustConnect()

	repo := model.New(mongo)
	assist := assistant.New()

	server := chat.NewServer(repo, assist)

	// Configure handler
	handler := mux.NewRouter()
	handler.Use(
		httpx.Tracing(), // Add tracing middleware (first to capture entire request)
		httpx.Logger(),
		httpx.Recovery(),
		httpx.Metrics(), // Add metrics middleware
	)

	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Hi, my name is Clippy!")
	})

	handler.PathPrefix("/twirp/").Handler(pb.NewChatServiceServer(server, twirp.WithServerJSONSkipDefaults(true)))

	// Create HTTP server with graceful shutdown support
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Channel to listen for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		slog.Info("Starting the server...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			panic(err)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Server stopped")
}
