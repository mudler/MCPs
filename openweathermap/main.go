package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Input struct {
	City string `json:"city" jsonschema:"the city to get the weather for"`
}

type Output struct {
	Temperature string     `json:"temperature" jsonschema:"current temperature"`
	Wind        string     `json:"wind" jsonschema:"wind speed"`
	Description string     `json:"description" jsonschema:"weather description"`
	Forecast    []Forecast `json:"forecast" jsonschema:"5 day weather forecast"`
}

type Forecast struct {
	Day         string `json:"day" jsonschema:"forecast day in YYYY-MM-DD format"`
	Temperature string `json:"temperature" jsonschema:"temperature for the day"`
	Wind        string `json:"wind" jsonschema:"wind speed for the day"`
}

type geocodeEntry struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type currentWeatherResponse struct {
	Main struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
}

type forecastResponse struct {
	List []struct {
		DateText string `json:"dt_txt"`
		Main     struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
	} `json:"list"`
}

func normalizeCity(city string) string {
	city = strings.TrimSpace(city)

	replacements := []string{
		" Texas", ", Texas",
		" TX", ", TX",
	}

	for _, replacement := range replacements {
		city = strings.ReplaceAll(city, replacement, "")
	}

	return strings.TrimSpace(city)
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func geocodeCity(ctx context.Context, client *http.Client, apiKey, city string) (float64, float64, error) {
	endpoint := fmt.Sprintf(
		"https://api.openweathermap.org/geo/1.0/direct?q=%s&limit=1&appid=%s",
		url.QueryEscape(city),
		apiKey,
	)

	var results []geocodeEntry
	if err := getJSON(ctx, client, endpoint, &results); err != nil {
		return 0, 0, err
	}

	if len(results) == 0 {
		return 0, 0, fmt.Errorf("location not found")
	}

	return results[0].Lat, results[0].Lon, nil
}

func getCoordinates(ctx context.Context, client *http.Client, apiKey, city string) (float64, float64, error) {
	attempts := []string{
		city,
		normalizeCity(city),
		strings.TrimSpace(strings.Split(city, ",")[0]),
	}

	seen := map[string]bool{}
	var lastErr error

	for _, attempt := range attempts {
		attempt = strings.TrimSpace(attempt)
		if attempt == "" || seen[attempt] {
			continue
		}
		seen[attempt] = true

		lat, lon, err := geocodeCity(ctx, client, apiKey, attempt)
		if err == nil {
			return lat, lon, nil
		}

		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("location not found")
	}

	return 0, 0, lastErr
}

func GetWeather(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	apiKey := os.Getenv("OWM_API_KEY")
	if apiKey == "" {
		return nil, Output{}, fmt.Errorf("OWM_API_KEY not set")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	lat, lon, err := getCoordinates(ctx, client, apiKey, input.City)
	if err != nil {
		return nil, Output{}, err
	}

	currentURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&appid=%s&units=imperial",
		lat, lon, apiKey,
	)

	var current currentWeatherResponse
	if err := getJSON(ctx, client, currentURL, &current); err != nil {
		return nil, Output{}, err
	}

	if len(current.Weather) == 0 {
		return nil, Output{}, fmt.Errorf("missing weather description in current conditions response")
	}

	forecastURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/forecast?lat=%f&lon=%f&appid=%s&units=imperial",
		lat, lon, apiKey,
	)

	var forecastData forecastResponse
	if err := getJSON(ctx, client, forecastURL, &forecastData); err != nil {
		return nil, Output{}, err
	}

	forecast := make([]Forecast, 0, 5)
	seenDays := map[string]bool{}

	for _, item := range forecastData.List {
		parts := strings.Split(item.DateText, " ")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		day := parts[0]
		if seenDays[day] {
			continue
		}

		forecast = append(forecast, Forecast{
			Day:         day,
			Temperature: fmt.Sprintf("%.1f F", item.Main.Temp),
			Wind:        fmt.Sprintf("%.1f mph", item.Wind.Speed),
		})
		seenDays[day] = true

		if len(forecast) == 5 {
			break
		}
	}

	output := Output{
		Temperature: fmt.Sprintf("%.1f F", current.Main.Temp),
		Wind:        fmt.Sprintf("%.1f mph", current.Wind.Speed),
		Description: current.Weather[0].Description,
		Forecast:    forecast,
	}

	return nil, output, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "openweathermap",
		Version: "v1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_weather",
		Description: "Get current weather and 5-day forecast for a city using OpenWeatherMap",
	}, GetWeather)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
