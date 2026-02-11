package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	processmanager "github.com/mudler/go-processmanager"
)

// Session represents an active opencode session
type Session struct {
	ID        string                  `json:"id"`
	Status    string                  `json:"status"`
	PID       string                  `json:"pid"`
	Message   string                  `json:"message"`
	Model     string                  `json:"model"`
	CreatedAt time.Time               `json:"created_at"`
	StartedAt time.Time               `json:"started_at,omitempty"`
	StoppedAt time.Time               `json:"stopped_at,omitempty"`
	ExitCode  string                  `json:"exit_code"`
	Process   *processmanager.Process `json:"-"`
	StateDir  string                  `json:"state_dir"`
}

// SessionManager manages all opencode sessions
type SessionManager struct {
	sessions    map[string]*Session
	mutex       sync.RWMutex
	sessionDir  string
	maxSessions int
}

// Global session manager
var globalSessionManager *SessionManager

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int with default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func main() {
	// Initialize session manager
	sessionDir := getEnv("OPENCODE_SESSION_DIR", "/tmp/opencode-sessions")
	maxSessions := getEnvInt("OPENCODE_MAX_SESSIONS", 10)

	// Ensure session directory exists
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		log.Fatalf("Failed to create session directory: %v", err)
	}

	globalSessionManager = &SessionManager{
		sessions:    make(map[string]*Session),
		sessionDir:  sessionDir,
		maxSessions: maxSessions,
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "opencode",
		Version: "v1.0.0",
	}, nil)

	// Register tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_session",
		Description: "Start a new opencode session with a message. Returns a session ID that can be used to check status and retrieve logs.",
	}, StartSessionHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_status",
		Description: "Get the status of an opencode session by ID. Returns running, completed, failed, or not_found.",
	}, GetSessionStatusHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_session_logs",
		Description: "Get the stdout and stderr logs from an opencode session. Optionally specify the number of lines to retrieve (default 100).",
	}, GetSessionLogsHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stop_session",
		Description: "Stop a running opencode session by ID. Optionally force kill the process.",
	}, StopSessionHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List all opencode sessions. Optionally filter by status: running, completed, failed, or all.",
	}, ListSessionsHandler)

	// Run server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

	// Cleanup all sessions on shutdown
	globalSessionManager.StopAllSessions()
}
