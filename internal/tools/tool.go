package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v2"
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

// Registry manages a collection of tools and provides methods to register,
// retrieve, and list them. It serves as the central hub for all available tools.
type Registry struct {
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
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name. Returns the tool and true if found, nil and false otherwise.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all tool definitions in a format suitable for OpenAI API calls
func (r *Registry) Definitions() []openai.ChatCompletionToolUnionParam {
	defs := make([]openai.ChatCompletionToolUnionParam, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute runs a tool by name with the provided arguments. Returns an error if the tool
// is not found or if execution fails.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}

// List returns the names of all registered tools
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
