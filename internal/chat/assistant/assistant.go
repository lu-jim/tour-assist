package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/tools"
	"github.com/openai/openai-go/v2"
)

type Assistant struct {
	cli          openai.Client
	toolRegistry *tools.Registry
}

func New() *Assistant {
	registry := tools.NewRegistry()
	return &Assistant{
		cli:          openai.NewClient(),
		toolRegistry: registry,
	}
}

func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	systemPrompt := "Return ONLY a concise 2–6 word title summarizing the user’s question. Do not answer the question. No punctuation or emojis. Max 80 chars."
	userMessage := conv.Messages[0].Content

	// Logging the system prompt and user message
	slog.InfoContext(ctx, "API Request",
		"system_prompt", systemPrompt,
		"user_message", userMessage)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT5Mini,
		Messages: msgs,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		slog.InfoContext(ctx, "Title API Request", "raw_title", resp.Choices[0].Message.Content)
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("empty response from OpenAI for title generation")
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "", errors.New("conversation has no messages")
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	// Register tools for this conversation
	registry := tools.NewRegistry()
	registry.Register(tools.NewGetWeatherTool(conv))
	registry.Register(tools.NewGetWeatherForecastTool(conv))
	registry.Register(tools.NewGetTodayDateTool())
	registry.Register(tools.NewGetHolidaysTool())

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses."),
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
		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4_1,
			Messages: msgs,
			Tools:    registry.Definitions(),
		})

		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices returned by OpenAI")
		}

		if message := resp.Choices[0].Message; len(message.ToolCalls) > 0 {
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

		return resp.Choices[0].Message.Content, nil
	}

	return "", errors.New("too many tool calls, unable to generate reply")
}
