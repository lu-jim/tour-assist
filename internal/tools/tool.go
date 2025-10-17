package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/openai/openai-go/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Tool represents a capability that the AI assistant can use to perform actions
// or retrieve information. Each tool has a name, description, OpenAI definition,
// and can execute with provided arguments.
type Tool interface {
	Name() string

	Description() string

	Definition() openai.ChatCompletionToolUnionParam

	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

const (
	meterName  = "github.com/acai-travel/tech-challenge/internal/tools"
	tracerName = "github.com/acai-travel/tech-challenge/internal/tools"
)

var (
	executionCounter  metric.Int64Counter
	durationHistogram metric.Float64Histogram
	errorCounter      metric.Int64Counter
)

func init() {
	meter := otel.Meter(meterName)

	var err error
	executionCounter, err = meter.Int64Counter(
		"tool.execution.count",
		metric.WithDescription("Total number of tool executions"),
		metric.WithUnit("{execution}"),
	)
	if err != nil {
		// If metric creation fails, the counter will be nil and won't record anything
		// This prevents the application from failing if metrics setup fails
	}

	durationHistogram, err = meter.Float64Histogram(
		"tool.execution.duration",
		metric.WithDescription("Duration of tool executions in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		// If metric creation fails, the histogram will be nil and won't record anything
	}

	errorCounter, err = meter.Int64Counter(
		"tool.execution.errors",
		metric.WithDescription("Total number of tool execution errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		// If metric creation fails, the counter will be nil and won't record anything
	}
}

// Registry manages a collection of tools and provides methods to register,
// retrieve, and list them. It serves as the central hub for all available tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry. If a tool with the same name already exists,
// it will be overwritten.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name. Returns the tool and true if found, nil and false otherwise.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all tool definitions in a format suitable for OpenAI API calls
func (r *Registry) Definitions() []openai.ChatCompletionToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]openai.ChatCompletionToolUnionParam, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute runs a tool by name with the provided arguments. Returns an error if the tool
// is not found or if execution fails.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	// Get tracer and start span
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Tool."+name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("tool.name", name),
		),
	)
	defer span.End()

	// Record start time for metrics
	startTime := time.Now()

	// Get tool
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		err := fmt.Errorf("unknown tool: %s", name)
		span.RecordError(err)
		span.SetStatus(codes.Error, "tool not found")

		// Record error metric
		if errorCounter != nil {
			errorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("tool.name", name),
				attribute.String("error.type", "not_found"),
			))
		}

		return "", err
	}

	// Execute tool
	result, err := tool.Execute(ctx, args)

	// Calculate duration
	duration := float64(time.Since(startTime).Milliseconds())

	// Prepare common attributes
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", name),
	}

	// Record metrics and span status
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "tool execution failed")

		// Increment error counter
		if errorCounter != nil {
			errorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("tool.name", name),
				attribute.String("error.type", "execution_failed"),
			))
		}
	} else {
		span.SetAttributes(attribute.Int("tool.result.length", len(result)))
		span.SetStatus(codes.Ok, "success")
	}

	// Record execution count
	if executionCounter != nil {
		executionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	// Record execution duration
	if durationHistogram != nil {
		durationHistogram.Record(ctx, duration, metric.WithAttributes(attrs...))
	}

	return result, err
}

// List returns the names of all registered tools
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
