package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultAPIURL            = "https://api.github.com"
	defaultMaxCommentLength  = 4000
	defaultHTTPClientTimeout = 30 * time.Second
)

var (
	client        *GitHubClient
	maxCommentLen = defaultMaxCommentLength
)

func main() {
	apiURL := os.Getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	apiURL = strings.TrimRight(apiURL, "/")

	if v := os.Getenv("GITHUB_MAX_COMMENT_LENGTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("GITHUB_MAX_COMMENT_LENGTH must be an integer: %v", err)
		}
		maxCommentLen = n
	}

	client = &GitHubClient{
		BaseURL: apiURL,
		Token:   os.Getenv("GITHUB_TOKEN"),
		HTTPClient: &http.Client{
			Timeout: defaultHTTPClientTimeout,
		},
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "github", Version: "v1.0.0"}, nil)

	enabled := enabledToolSet(os.Getenv("GITHUB_TOOLS"))

	if enabled == nil || enabled["get_issue"] {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_issue",
			Description: "Fetch a GitHub issue including its title, body, state, author, labels, and comments. Accepts either a full issue URL or owner/repo/number.",
		}, GetIssue)
	}

	if enabled == nil || enabled["get_pull_request"] {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "get_pull_request",
			Description: "Fetch a GitHub pull request including title, body, state, author, base/head refs, labels, and discussion comments. Set include_diff=true to also return the unified diff of the PR.",
		}, GetPullRequest)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func enabledToolSet(env string) map[string]bool {
	env = strings.TrimSpace(env)
	if env == "" || env == "all" {
		return nil
	}
	set := make(map[string]bool)
	for _, name := range strings.Split(env, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			set[name] = true
		}
	}
	return set
}
