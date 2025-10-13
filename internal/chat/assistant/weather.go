package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
