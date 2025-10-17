package httpx

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/acai-travel/tech-challenge/internal/httpx"

// metricsMiddleware holds the OpenTelemetry metrics instruments
type metricsMiddleware struct {
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorCounter    metric.Int64Counter
}

// newMetricsMiddleware creates a new metrics middleware with initialized instruments
func newMetricsMiddleware() (*metricsMiddleware, error) {
	meter := otel.Meter(meterName)

	requestCounter, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	errorCounter, err := meter.Int64Counter(
		"http.server.request.errors",
		metric.WithDescription("Total number of HTTP errors (4xx and 5xx responses)"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &metricsMiddleware{
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
		errorCounter:    errorCounter,
	}, nil
}

// Metrics returns a middleware that captures HTTP request metrics
func Metrics() func(handler http.Handler) http.Handler {
	// Initialize metrics middleware
	mm, err := newMetricsMiddleware()
	if err != nil {
		// If we can't initialize metrics, return a no-op middleware
		// This prevents the application from failing if metrics setup fails
		return func(handler http.Handler) http.Handler {
			return handler
		}
	}

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Wrap the response writer to capture status code
			saw := &statusAwareResponseWriter{ResponseWriter: w}

			// Execute the handler
			handler.ServeHTTP(saw, r)

			// Calculate duration in milliseconds
			duration := float64(time.Since(startTime).Milliseconds())

			// Prepare common attributes
			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.Int("http.status_code", saw.status),
			}

			// Record request count
			mm.requestCounter.Add(r.Context(), 1, metric.WithAttributes(attrs...))

			// Record request duration
			mm.requestDuration.Record(r.Context(), duration, metric.WithAttributes(attrs...))

			// Record errors for 4xx and 5xx responses
			if saw.status >= 400 {
				errorAttrs := []attribute.KeyValue{
					attribute.String("http.method", r.Method),
					attribute.String("http.path", r.URL.Path),
					attribute.String("http.status_code", strconv.Itoa(saw.status)),
				}
				mm.errorCounter.Add(r.Context(), 1, metric.WithAttributes(errorAttrs...))
			}
		})
	}
}
