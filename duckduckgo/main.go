package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/tools/duckduckgo"
)

type Input struct {
	Query string `json:"query" jsonschema:"the query to search for"`
}

type Output struct {
	Result string `json:"result" jsonschema:"the result of the search"`
}

var maxResults = 5

func init() {
	var err error
	maxResults, err = strconv.Atoi(os.Getenv("MAX_RESULTS"))
	if err != nil {
		maxResults = 5
	}
}

func Search(ctx context.Context, req *mcp.CallToolRequest, input Input) (
	*mcp.CallToolResult,
	Output,
	error,
) {
	ddg, err := duckduckgo.New(maxResults, "MCP")
	if err != nil {
		return nil, Output{Result: "Error searching the web"}, err
	}
	result, err := ddg.Call(context.Background(), input.Query)
	if err != nil {
		return nil, Output{Result: "Error searching the web"}, err
	}

	return nil, Output{Result: result}, nil
}

func main() {
	// Create a server with a single tool.
	server := mcp.NewServer(&mcp.Implementation{Name: "duckduckgo", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "search", Description: "search the web"}, Search)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
