package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/openai/openai-go/v2"
)

// AmadeusTokenResponse represents the OAuth2 token response from Amadeus API
type AmadeusTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// FlightDestination represents a single flight destination with price
type FlightDestination struct {
	Origin        string
	Destination   string
	DepartureDate string
	ReturnDate    string
	Price         string
}

// FetchAmadeusToken retrieves an OAuth2 access token from Amadeus API
func FetchAmadeusToken(ctx context.Context, httpClient *http.Client, apiKey, apiSecret string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("missing AMADEUS_API_KEY")
	}
	if apiSecret == "" {
		return "", fmt.Errorf("missing AMADEUS_API_SECRET")
	}

	u := "https://test.api.amadeus.com/v1/security/oauth2/token"

	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", apiKey)
	formData.Set("client_secret", apiSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to extract error from OAuth response
		var oauthErr struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthErr)
		if oauthErr.Error != "" {
			errMsg := oauthErr.Error
			if oauthErr.ErrorDescription != "" {
				errMsg = fmt.Sprintf("%s: %s", oauthErr.Error, oauthErr.ErrorDescription)
			}
			return "", fmt.Errorf("authentication failed: %s", errMsg)
		}
		return "", fmt.Errorf("authentication failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp AmadeusTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token received")
	}

	return tokenResp.AccessToken, nil
}

// FetchFlightDestinations calls Amadeus flight-offers endpoint
func FetchFlightDestinations(ctx context.Context, httpClient *http.Client, token, origin, destination, departureDate string, maxPrice int) ([]FlightDestination, error) {
	if token == "" {
		return nil, fmt.Errorf("missing access token")
	}
	if origin == "" {
		return nil, fmt.Errorf("missing origin")
	}
	if destination == "" {
		return nil, fmt.Errorf("missing destination")
	}
	if departureDate == "" {
		return nil, fmt.Errorf("missing departure date")
	}

	u := url.URL{Scheme: "https", Host: "test.api.amadeus.com", Path: "/v2/shopping/flight-offers"}
	q := u.Query()
	q.Set("originLocationCode", origin)
	q.Set("destinationLocationCode", destination)
	q.Set("departureDate", departureDate)
	q.Set("adults", "1")
	q.Set("max", "10") // Limit to 10 offers
	if maxPrice > 0 {
		q.Set("maxPrice", fmt.Sprintf("%d", maxPrice))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

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
		// Try to extract error message from Amadeus API response
		var apiErr struct {
			Errors []struct {
				Detail string `json:"detail"`
				Title  string `json:"title"`
				Status int    `json:"status"`
				Code   int    `json:"code"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if len(apiErr.Errors) > 0 {
			errMsg := apiErr.Errors[0].Detail
			if errMsg == "" {
				errMsg = apiErr.Errors[0].Title
			}
			if errMsg != "" {
				return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, errMsg)
			}
		}
		return nil, fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Data []struct {
			Type        string `json:"type"`
			ID          string `json:"id"`
			Itineraries []struct {
				Duration string `json:"duration"`
				Segments []struct {
					Departure struct {
						IataCode string `json:"iataCode"`
						At       string `json:"at"`
					} `json:"departure"`
					Arrival struct {
						IataCode string `json:"iataCode"`
						At       string `json:"at"`
					} `json:"arrival"`
					CarrierCode string `json:"carrierCode"`
					Number      string `json:"number"`
				} `json:"segments"`
			} `json:"itineraries"`
			Price struct {
				Currency string `json:"currency"`
				Total    string `json:"total"`
				Base     string `json:"base"`
			} `json:"price"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := make([]FlightDestination, 0, len(data.Data))
	for _, offer := range data.Data {
		if len(offer.Itineraries) == 0 || len(offer.Itineraries[0].Segments) == 0 {
			continue
		}

		firstSegment := offer.Itineraries[0].Segments[0]
		lastSegment := offer.Itineraries[0].Segments[len(offer.Itineraries[0].Segments)-1]

		out = append(out, FlightDestination{
			Origin:        firstSegment.Departure.IataCode,
			Destination:   lastSegment.Arrival.IataCode,
			DepartureDate: firstSegment.Departure.At,
			ReturnDate:    "", // One-way flights for now
			Price:         offer.Price.Total + " " + offer.Price.Currency,
		})
	}
	return out, nil
}

// GetFlightPricesTool retrieves flight destinations and prices
type GetFlightPricesTool struct {
	httpClient *http.Client
	conv       *model.Conversation
}

func NewGetFlightPricesTool(conv *model.Conversation) *GetFlightPricesTool {
	return &GetFlightPricesTool{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		conv:       conv,
	}
}

func (t *GetFlightPricesTool) Name() string {
	return "get_flight_prices"
}

func (t *GetFlightPricesTool) Description() string {
	return "Search for flight offers and prices between two cities on a specific date. Returns available flights with pricing information."
}

func (t *GetFlightPricesTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        t.Name(),
		Description: openai.String(t.Description()),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"origin": map[string]string{
					"type":        "string",
					"description": "IATA code of the origin airport (e.g., 'BCN' for Barcelona, 'NYC' for New York)",
				},
				"destination": map[string]string{
					"type":        "string",
					"description": "IATA code of the destination airport (e.g., 'MAD' for Madrid, 'LON' for London)",
				},
				"departureDate": map[string]string{
					"type":        "string",
					"description": "Departure date in YYYY-MM-DD format (e.g., '2025-10-18')",
				},
				"maxPrice": map[string]any{
					"type":        "integer",
					"description": "Maximum price per traveler in the currency of the origin country (optional)",
				},
			},
			"required": []string{"origin", "destination", "departureDate"},
		},
	})
}

func (t *GetFlightPricesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Origin        string `json:"origin"`
		Destination   string `json:"destination"`
		DepartureDate string `json:"departureDate"`
		MaxPrice      int    `json:"maxPrice"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	origin := strings.TrimSpace(strings.ToUpper(payload.Origin))
	if origin == "" {
		return "", fmt.Errorf("origin is required")
	}

	destination := strings.TrimSpace(strings.ToUpper(payload.Destination))
	if destination == "" {
		return "", fmt.Errorf("destination is required")
	}

	departureDate := strings.TrimSpace(payload.DepartureDate)
	if departureDate == "" {
		return "", fmt.Errorf("departure date is required")
	}

	maxPrice := payload.MaxPrice

	// Get API credentials
	apiKey := os.Getenv("AMADEUS_API_KEY")
	apiSecret := os.Getenv("AMADEUS_API_SECRET")

	// Check if credentials are set
	if apiKey == "" || apiSecret == "" {
		return "", fmt.Errorf("amadeus API credentials not configured - please set AMADEUS_API_KEY and AMADEUS_API_SECRET environment variables")
	}

	// Fetch OAuth2 token
	token, err := FetchAmadeusToken(ctx, t.httpClient, apiKey, apiSecret)
	if err != nil {
		return "", fmt.Errorf("flight search failed: %w", err)
	}

	// Fetch flight destinations
	flights, err := FetchFlightDestinations(ctx, t.httpClient, token, origin, destination, departureDate, maxPrice)
	if err != nil {
		return "", fmt.Errorf("flight search failed: %w", err)
	}

	if len(flights) == 0 {
		return "No flights found matching your criteria.", nil
	}

	// Format response
	lines := make([]string, 0, len(flights)+1)
	header := fmt.Sprintf("Found %d flight option%s from %s to %s on %s:",
		len(flights),
		map[bool]string{true: "s", false: ""}[len(flights) != 1],
		origin,
		destination,
		departureDate)
	lines = append(lines, header)

	for i, f := range flights {
		// Extract time from ISO datetime (2025-10-18T14:30:00)
		depTime := f.DepartureDate
		if len(depTime) >= 16 {
			depTime = depTime[11:16] // Extract HH:MM
		}

		flightInfo := fmt.Sprintf("%d. %s → %s at %s: %s",
			i+1,
			f.Origin,
			f.Destination,
			depTime,
			f.Price)
		lines = append(lines, flightInfo)
	}

	return strings.Join(lines, "\n"), nil
}
