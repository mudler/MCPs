package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	Forecast    []Forecast `json:"forecast" jsonschema:"weather forecast"`
}

type Forecast struct {
	Day         string `json:"day" jsonschema:"day number"`
	Temperature string `json:"temperature" jsonschema:"temperature for the day"`
	Wind        string `json:"wind" jsonschema:"wind speed for the day"`
}

type WeatherAPIResponse struct {
	Temperature string     `json:"temperature"`
	Wind        string     `json:"wind"`
	Description string     `json:"description"`
	Forecast    []Forecast `json:"forecast"`
}

func GetWeather(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	// URL encode the city name to handle special characters and spaces
	encodedCity := url.QueryEscape(input.City)
	weatherURL := fmt.Sprintf("http://goweather.xyz/weather/%s", encodedCity)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make HTTP request
	resp, err := client.Get(weatherURL)
	if err != nil {
		return nil, Output{}, fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer resp.Body.Close()

	// Check if request was successful
	if resp.StatusCode != http.StatusOK {
		return nil, Output{}, fmt.Errorf("weather API returned status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Output{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var weatherResp WeatherAPIResponse
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		return nil, Output{}, fmt.Errorf("failed to parse weather data: %w", err)
	}

	// Convert to output format
	output := Output{
		Temperature: weatherResp.Temperature,
		Wind:        weatherResp.Wind,
		Description: weatherResp.Description,
		Forecast:    weatherResp.Forecast,
	}

	return nil, output, nil
}

func main() {
	// Create a server with a single tool.
	server := mcp.NewServer(&mcp.Implementation{Name: "weather", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_weather", Description: "Get current weather and forecast for a city"}, GetWeather)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
