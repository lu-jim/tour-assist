package httpx

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/acai-travel/tech-challenge/internal/httpx"

// Tracing returns a middleware that creates traces for HTTP requests
func Tracing() func(handler http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)
	propagator := otel.GetTextMapPropagator()

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start a new span for this HTTP request
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.path", r.URL.Path),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
				),
			)
			defer span.End()

			// Wrap the response writer to capture status code
			saw := &statusAwareResponseWriter{ResponseWriter: w}

			// Execute the handler with the trace context
			handler.ServeHTTP(saw, r.WithContext(ctx))

			// Record the status code
			span.SetAttributes(attribute.Int("http.status_code", saw.status))

			// Mark span as error if status is 5xx
			if saw.status >= 500 {
				span.SetStatus(codes.Error, "server error")
			} else if saw.status >= 400 {
				span.SetStatus(codes.Error, "client error")
			} else {
				span.SetStatus(codes.Ok, "success")
			}
		})
	}
}
