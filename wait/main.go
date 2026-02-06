package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Input struct {
	Duration float64 `json:"duration" jsonschema:"duration to wait in seconds (supports fractional seconds like 0.5, 1.5)"`
}

type Output struct {
	Message string `json:"message" jsonschema:"confirmation message indicating the wait completed"`
}

const maxDuration = 3600 // 1 hour in seconds

func Wait(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	// Validate input
	if input.Duration <= 0 {
		return nil, Output{}, fmt.Errorf("duration must be positive, got: %f", input.Duration)
	}
	if input.Duration > maxDuration {
		return nil, Output{}, fmt.Errorf("duration exceeds maximum of %d seconds (1 hour), got: %f", maxDuration, input.Duration)
	}

	// Convert seconds to duration
	duration := time.Duration(input.Duration * float64(time.Second))

	// Wait with context cancellation support
	select {
	case <-ctx.Done():
		return nil, Output{}, ctx.Err()
	case <-time.After(duration):
		message := fmt.Sprintf("Waited for %.2f seconds", input.Duration)
		return nil, Output{Message: message}, nil
	}
}

func main() {
	// Create a server with a single tool.
	server := mcp.NewServer(&mcp.Implementation{Name: "wait", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "wait", Description: "Wait for a specified duration in seconds"}, Wait)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
