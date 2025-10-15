package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/openai/openai-go/v2"
)

// LoadCalendar loads calendar events from a URL
func LoadCalendar(ctx context.Context, link string) ([]*ics.VEvent, error) {
	slog.InfoContext(ctx, "Loading calendar", "link", link)

	cal, err := ics.ParseCalendarFromUrl(link, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse calendar: %w", err)
	}

	return cal.Events(), nil
}

// GetTodayDateTool returns today's date and time in RFC3339 format
type GetTodayDateTool struct{}

func NewGetTodayDateTool() *GetTodayDateTool {
	return &GetTodayDateTool{}
}

func (t *GetTodayDateTool) Name() string {
	return "get_today_date"
}

func (t *GetTodayDateTool) Description() string {
	return "Get today's date and time in RFC3339 format"
}

func (t *GetTodayDateTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name(),
		Description: openai.String(t.Description()),
	})
}

func (t *GetTodayDateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

// GetHolidaysTool retrieves local bank and public holidays
type GetHolidaysTool struct{}

func NewGetHolidaysTool() *GetHolidaysTool {
	return &GetHolidaysTool{}
}

func (t *GetHolidaysTool) Name() string {
	return "get_holidays"
}

func (t *GetHolidaysTool) Description() string {
	return "Gets local bank and public holidays. Each line is a single holiday in the format 'YYYY-MM-DD: Holiday Name'."
}

func (t *GetHolidaysTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name(),
		Description: openai.String(t.Description()),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"before_date": map[string]string{
					"type":        "string",
					"description": "Optional date in RFC3339 format to get holidays before this date. If not provided, all holidays will be returned.",
				},
				"after_date": map[string]string{
					"type":        "string",
					"description": "Optional date in RFC3339 format to get holidays after this date. If not provided, all holidays will be returned.",
				},
				"max_count": map[string]string{
					"type":        "integer",
					"description": "Optional maximum number of holidays to return. If not provided, all holidays will be returned.",
				},
			},
		},
	})
}

func (t *GetHolidaysTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	link := "https://www.officeholidays.com/ics/spain/catalonia"
	if v := os.Getenv("HOLIDAY_CALENDAR_LINK"); v != "" {
		link = v
	}

	events, err := LoadCalendar(ctx, link)
	if err != nil {
		return "", fmt.Errorf("failed to load holiday events: %w", err)
	}

	// Pre-collect dates and sort to ensure deterministic order
	type datedEvent struct {
		evt  *ics.VEvent
		date time.Time
	}
	var datedEvents []datedEvent
	for _, ev := range events {
		d, err := ev.GetAllDayStartAt()
		if err != nil {
			continue
		}
		datedEvents = append(datedEvents, datedEvent{evt: ev, date: d})
	}
	sort.Slice(datedEvents, func(i, j int) bool { return datedEvents[i].date.Before(datedEvents[j].date) })

	var payload struct {
		BeforeDate time.Time `json:"before_date,omitempty"`
		AfterDate  time.Time `json:"after_date,omitempty"`
		MaxCount   int       `json:"max_count,omitempty"`
	}

	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	var holidays []string
	for _, item := range datedEvents {
		if payload.MaxCount > 0 && len(holidays) >= payload.MaxCount {
			break
		}

		date := item.date

		if !payload.BeforeDate.IsZero() && date.After(payload.BeforeDate) {
			continue
		}

		if !payload.AfterDate.IsZero() && date.Before(payload.AfterDate) {
			continue
		}

		summary := "No summary"
		if p := item.evt.GetProperty(ics.ComponentPropertySummary); p != nil {
			summary = p.Value
		}
		holidays = append(holidays, date.Format(time.DateOnly)+": "+summary)
	}

	return strings.Join(holidays, "\n"), nil
}
