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
	"go.opentelemetry.io/otel/sdk/metric"
)

// initMeterProvider initializes an OpenTelemetry MeterProvider with a stdout exporter
func initMeterProvider() (*metric.MeterProvider, error) {
	// Create stdout exporter
	exporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	// Create a meter provider with a periodic reader
	// The reader will export metrics every 10 seconds
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter,
			metric.WithInterval(10*time.Second))),
	)

	// Set the global meter provider
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func main() {
	// Initialize OpenTelemetry meter provider
	meterProvider, err := initMeterProvider()
	if err != nil {
		slog.Error("Failed to initialize meter provider", "error", err)
		panic(err)
	}

	// Setup graceful shutdown for metrics
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := meterProvider.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown meter provider", "error", err)
		}
	}()

	mongo := mongox.MustConnect()

	repo := model.New(mongo)
	assist := assistant.New()

	server := chat.NewServer(repo, assist)

	// Configure handler
	handler := mux.NewRouter()
	handler.Use(
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
