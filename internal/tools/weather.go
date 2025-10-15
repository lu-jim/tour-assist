package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/openai/openai-go/v2"
)

// CurrentWeather holds a concise snapshot of current conditions in Celsius units.
type CurrentWeather struct {
	Location   string
	Condition  string
	TempC      float64
	FeelsLikeC float64
	WindKph    float64
	Humidity   int
}

// ForecastDay represents a single day's forecast in Celsius units.
type ForecastDay struct {
	Date         string
	Condition    string
	MaxC         float64
	MinC         float64
	ChanceOfRain int
}

// IntOrString handles JSON fields that may be number or quoted string.
type IntOrString int

func (v *IntOrString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*v = 0
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return err
		}
		if s == "" {
			*v = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*v = IntOrString(n)
		return nil
	}
	var i int
	if err := json.Unmarshal(b, &i); err == nil {
		*v = IntOrString(i)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*v = IntOrString(int(f))
		return nil
	}
	return fmt.Errorf("unexpected value for IntOrString: %s", string(b))
}

// FetchCurrentWeather calls WeatherAPI current weather endpoint for a location.
func FetchCurrentWeather(ctx context.Context, httpClient *http.Client, apiKey, location string) (CurrentWeather, error) {
	var zero CurrentWeather
	if apiKey == "" {
		return zero, fmt.Errorf("missing WEATHER_API_KEY")
	}
	if location == "" {
		return zero, fmt.Errorf("missing location")
	}

	u := url.URL{Scheme: "https", Host: "api.weatherapi.com", Path: "/v1/current.json"}
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("q", location)
	q.Set("aqi", "no")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return zero, fmt.Errorf("build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// WeatherAPI provides error.message field on failures
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Error.Message != "" {
			return zero, fmt.Errorf("api error: %s", apiErr.Error.Message)
		}
		return zero, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	var data struct {
		Location struct {
			Name string `json:"name"`
		} `json:"location"`
		Current struct {
			TempC      float64 `json:"temp_c"`
			FeelslikeC float64 `json:"feelslike_c"`
			WindKph    float64 `json:"wind_kph"`
			Humidity   int     `json:"humidity"`
			Condition  struct {
				Text string `json:"text"`
			} `json:"condition"`
		} `json:"current"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}

	return CurrentWeather{
		Location:   data.Location.Name,
		Condition:  data.Current.Condition.Text,
		TempC:      data.Current.TempC,
		FeelsLikeC: data.Current.FeelslikeC,
		WindKph:    data.Current.WindKph,
		Humidity:   data.Current.Humidity,
	}, nil
}

// FetchForecast calls WeatherAPI forecast endpoint for a location and number of days.
func FetchForecast(ctx context.Context, httpClient *http.Client, apiKey, location string, days int) ([]ForecastDay, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("missing WEATHER_API_KEY")
	}
	if location == "" {
		return nil, fmt.Errorf("missing location")
	}
	if days <= 0 {
		days = 1
	}

	u := url.URL{Scheme: "https", Host: "api.weatherapi.com", Path: "/v1/forecast.json"}
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("q", location)
	q.Set("days", strconv.Itoa(days))
	q.Set("aqi", "no")
	q.Set("alerts", "no")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Error.Message != "" {
			return nil, fmt.Errorf("api error: %s", apiErr.Error.Message)
		}
		return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	var data struct {
		Forecast struct {
			Forecastday []struct {
				Date string `json:"date"`
				Day  struct {
					MaxtempC          float64     `json:"maxtemp_c"`
					MintempC          float64     `json:"mintemp_c"`
					DailyChanceOfRain IntOrString `json:"daily_chance_of_rain"`
					Condition         struct {
						Text string `json:"text"`
					} `json:"condition"`
				} `json:"day"`
			} `json:"forecastday"`
		} `json:"forecast"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]ForecastDay, 0, len(data.Forecast.Forecastday))
	for _, fd := range data.Forecast.Forecastday {
		out = append(out, ForecastDay{
			Date:         fd.Date,
			Condition:    fd.Day.Condition.Text,
			MaxC:         fd.Day.MaxtempC,
			MinC:         fd.Day.MintempC,
			ChanceOfRain: int(fd.Day.DailyChanceOfRain),
		})
	}
	return out, nil
}

// GetWeatherTool retrieves current weather for a location
type GetWeatherTool struct {
	httpClient *http.Client
	conv       *model.Conversation
}

func NewGetWeatherTool(conv *model.Conversation) *GetWeatherTool {
	return &GetWeatherTool{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		conv:       conv,
	}
}

func (t *GetWeatherTool) Name() string {
	return "get_weather"
}

func (t *GetWeatherTool) Description() string {
	return "Get weather at the given location"
}

func (t *GetWeatherTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name(),
		Description: openai.String(t.Description()),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
			},
			"required": []string{"location"},
		},
	})
}

func (t *GetWeatherTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	// Resolve location: payload -> parse last user message -> env default
	resolvedLocation := strings.TrimSpace(payload.Location)
	if resolvedLocation == "" {
		// try to parse "weather in X" from last user message
		for i := len(t.conv.Messages) - 1; i >= 0; i-- {
			if t.conv.Messages[i].Role == model.RoleUser {
				re := regexp.MustCompile(`(?i)weather\s+in\s+([^?.!]+)`) // naive extraction
				if m := re.FindStringSubmatch(t.conv.Messages[i].Content); len(m) == 2 {
					resolvedLocation = strings.TrimSpace(m[1])
					break
				}
			}
		}
	}
	if resolvedLocation == "" {
		resolvedLocation = os.Getenv("WEATHER_DEFAULT_LOCATION")
	}
	if resolvedLocation == "" {
		return "", fmt.Errorf("weather lookup failed: please provide a location (e.g., 'weather in Paris')")
	}

	apiKey := os.Getenv("WEATHER_API_KEY")
	cw, err := FetchCurrentWeather(ctx, t.httpClient, apiKey, resolvedLocation)
	if err != nil {
		return "", fmt.Errorf("weather lookup failed: %w", err)
	}
	name := cw.Location
	if name == "" {
		name = resolvedLocation
	}
	result := fmt.Sprintf("%s: %.0f°C, %s. Feels %.0f°C. Wind %.0f kph. Humidity %d%%", name, cw.TempC, cw.Condition, cw.FeelsLikeC, cw.WindKph, cw.Humidity)
	return result, nil
}

// GetWeatherForecastTool retrieves weather forecast for a location
type GetWeatherForecastTool struct {
	httpClient *http.Client
	conv       *model.Conversation
}

func NewGetWeatherForecastTool(conv *model.Conversation) *GetWeatherForecastTool {
	return &GetWeatherForecastTool{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		conv:       conv,
	}
}

func (t *GetWeatherForecastTool) Name() string {
	return "get_weather_forecast"
}

func (t *GetWeatherForecastTool) Description() string {
	return "Get forecast for the given location"
}

func (t *GetWeatherForecastTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name(),
		Description: openai.String(t.Description()),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type": "string",
				},
				"days": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 7,
				},
			},
			"required": []string{"location"},
		},
	})
}

func (t *GetWeatherForecastTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Location string `json:"location"`
		Days     int    `json:"days"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	resolvedLocation := strings.TrimSpace(payload.Location)
	if resolvedLocation == "" {
		for i := len(t.conv.Messages) - 1; i >= 0; i-- {
			if t.conv.Messages[i].Role == model.RoleUser {
				// try to catch phrases like "forecast for X" or "X forecast"
				re := regexp.MustCompile(`(?i)(?:forecast\s+(?:for\s+)?)?in\s+([^?.!]+)|(?i)([^?.!]+)\s+forecast`)
				if m := re.FindStringSubmatch(t.conv.Messages[i].Content); len(m) >= 2 {
					cand := strings.TrimSpace(m[1])
					if cand == "" && len(m) >= 3 {
						cand = strings.TrimSpace(m[2])
					}
					if cand != "" {
						resolvedLocation = cand
						break
					}
				}
			}
		}
	}
	if resolvedLocation == "" {
		resolvedLocation = os.Getenv("WEATHER_DEFAULT_LOCATION")
	}
	if resolvedLocation == "" {
		return "", fmt.Errorf("forecast lookup failed: please provide a location (e.g., '3-day forecast for Barcelona')")
	}

	days := payload.Days
	if days <= 0 {
		if v := strings.TrimSpace(os.Getenv("WEATHER_FORECAST_DAYS")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				days = n
			}
		}
		if days <= 0 {
			days = 3
		}
	}
	if days < 1 {
		days = 1
	}
	if days > 7 {
		days = 7
	}

	apiKey := os.Getenv("WEATHER_API_KEY")
	fds, err := FetchForecast(ctx, t.httpClient, apiKey, resolvedLocation, days)
	if err != nil {
		return "", fmt.Errorf("forecast lookup failed: %w", err)
	}
	// Build a concise multi-line summary
	lines := make([]string, 0, len(fds)+1)
	lines = append(lines, fmt.Sprintf("%s forecast (%d day%s):", resolvedLocation, len(fds), map[bool]string{true: "s", false: ""}[len(fds) != 1]))
	for _, d := range fds {
		lines = append(lines, fmt.Sprintf("%s: %s, %.0f–%.0f°C, rain %d%%", d.Date, d.Condition, d.MinC, d.MaxC, d.ChanceOfRain))
	}
	return strings.Join(lines, "\n"), nil
}
