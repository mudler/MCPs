package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Input struct {
	Message string `json:"message" jsonschema:"the message to think about - the model should process and analyze this message"`
}

type Output struct {
	Result string `json:"result" jsonschema:"the result of thinking about the message"`
}

func Think(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	// Validate input
	if input.Message == "" {
		return nil, Output{}, fmt.Errorf("message cannot be empty")
	}

	// Simple no-op: just echo back the message
	// This forces the model to think by processing and echoing the message
	result := fmt.Sprintf("Thinking about: %s", input.Message)

	return nil, Output{Result: result}, nil
}

func main() {
	// Create a server with a single tool.
	server := mcp.NewServer(&mcp.Implementation{Name: "think", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "think", Description: "A no-op tool that forces the model to think about a message"}, Think)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
