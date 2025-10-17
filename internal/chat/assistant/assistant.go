package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/tools"
	"github.com/openai/openai-go/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Assistant struct {
	cli           openai.Client
	buildRegistry func(conv *model.Conversation) *tools.Registry
}

func New() *Assistant {
	return &Assistant{
		cli: openai.NewClient(),
		buildRegistry: func(conv *model.Conversation) *tools.Registry {
			r := tools.NewRegistry()
			r.Register(tools.NewGetWeatherTool(conv))
			r.Register(tools.NewGetWeatherForecastTool(conv))
			r.Register(tools.NewGetTodayDateTool())
			r.Register(tools.NewGetHolidaysTool())
			r.Register(tools.NewGetFlightPricesTool(conv))
			return r
		},
	}
}

// NewWithRegistryFactory allows injecting a custom per-conversation registry builder.
func NewWithRegistryFactory(build func(*model.Conversation) *tools.Registry) *Assistant {
	return &Assistant{
		cli:           openai.NewClient(),
		buildRegistry: build,
	}
}

func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/assistant")
	ctx, span := tracer.Start(ctx, "Assistant.Title",
		trace.WithAttributes(
			attribute.String("conversation.id", conv.ID.Hex()),
			attribute.Int("conversation.message_count", len(conv.Messages)),
		),
	)
	defer span.End()

	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	systemPrompt := "Return ONLY a concise 2–6 word title summarizing the user's question. Do not answer the question. No punctuation or emojis. Max 80 chars."
	userMessage := conv.Messages[0].Content

	// Logging the system prompt and user message
	slog.InfoContext(ctx, "API Request",
		"system_prompt", systemPrompt,
		"user_message", userMessage)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	// Create a child span for the OpenAI API call
	_, apiSpan := tracer.Start(ctx, "OpenAI.ChatCompletion.Title",
		trace.WithAttributes(
			attribute.String("openai.model", string(openai.ChatModelGPT5Nano)),
			attribute.Int("openai.messages", len(msgs)),
		),
	)

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT5,
		Messages: msgs,
	})

	apiSpan.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "OpenAI API call failed")
		return "", err
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		err := errors.New("empty response from OpenAI for title generation")
		span.RecordError(err)
		span.SetStatus(codes.Error, "empty response")
		return "", err
	}

	slog.InfoContext(ctx, "Title API Response", "raw_title", resp.Choices[0].Message.Content)

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		err := errors.New("empty response from OpenAI for title generation")
		span.RecordError(err)
		span.SetStatus(codes.Error, "empty response")
		return "", err
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	span.SetAttributes(attribute.String("title.generated", title))
	span.SetStatus(codes.Ok, "title generated successfully")

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/assistant")
	ctx, span := tracer.Start(ctx, "Assistant.Reply",
		trace.WithAttributes(
			attribute.String("conversation.id", conv.ID.Hex()),
			attribute.Int("conversation.message_count", len(conv.Messages)),
		),
	)
	defer span.End()

	if len(conv.Messages) == 0 {
		err := errors.New("conversation has no messages")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no messages")
		return "", err
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	// Build a per-conversation registry
	registry := a.buildRegistry(conv)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses. For time-sensitive queries (flights, weather forecasts, holidays, etc.), always use the get_today_date tool first to ensure you have the correct current date before making other API calls."),
	}

	for _, m := range conv.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	for i := 0; i < 15; i++ {
		// Create a child span for each OpenAI API call iteration
		_, iterSpan := tracer.Start(ctx, "OpenAI.ChatCompletion.Reply",
			trace.WithAttributes(
				attribute.String("openai.model", string(openai.ChatModelGPT5Mini)),
				attribute.Int("openai.messages", len(msgs)),
				attribute.Int("iteration", i),
			),
		)

		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4_1,
			Messages: msgs,
			Tools:    registry.Definitions(),
		})

		if err != nil {
			iterSpan.RecordError(err)
			iterSpan.SetStatus(codes.Error, "API call failed")
			iterSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "OpenAI API call failed")
			return "", err
		}

		if len(resp.Choices) == 0 {
			err := errors.New("no choices returned by OpenAI")
			iterSpan.RecordError(err)
			iterSpan.SetStatus(codes.Error, "no choices")
			iterSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, "no choices")
			return "", err
		}

		if message := resp.Choices[0].Message; len(message.ToolCalls) > 0 {
			iterSpan.SetAttributes(attribute.Int("tool_calls.count", len(message.ToolCalls)))
			iterSpan.End()

			msgs = append(msgs, message.ToParam())

			for _, call := range message.ToolCalls {
				slog.InfoContext(ctx, "Tool call received", "name", call.Function.Name, "args", call.Function.Arguments)

				result, err := registry.Execute(ctx, call.Function.Name, []byte(call.Function.Arguments))
				if err != nil {
					slog.ErrorContext(ctx, "Tool execution failed", "tool", call.Function.Name, "error", err)
					msgs = append(msgs, openai.ToolMessage(err.Error(), call.ID))
				} else {
					msgs = append(msgs, openai.ToolMessage(result, call.ID))
				}
			}

			continue
		}

		iterSpan.SetStatus(codes.Ok, "reply generated")
		iterSpan.End()

		reply := resp.Choices[0].Message.Content
		span.SetAttributes(
			attribute.String("reply.content", reply),
			attribute.Int("iterations", i+1),
		)
		span.SetStatus(codes.Ok, "reply generated successfully")
		return reply, nil
	}

	err := errors.New("too many tool calls, unable to generate reply")
	span.RecordError(err)
	span.SetStatus(codes.Error, "too many iterations")
	return "", err
}
