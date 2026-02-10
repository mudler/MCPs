package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StartSessionInput represents the input for starting a session
type StartSessionInput struct {
	Message   string   `json:"message" jsonschema:"the message to send to opencode"`
	Files     []string `json:"files,omitempty" jsonschema:"file(s) to attach to message"`
	Title     string   `json:"title,omitempty" jsonschema:"title for the session"`
	Continue  bool     `json:"continue,omitempty" jsonschema:"continue the last session"`
	SessionID string   `json:"session_id,omitempty" jsonschema:"session id to continue"`
	Thinking  bool     `json:"thinking,omitempty" jsonschema:"show thinking blocks"`
}

// StartSessionOutput represents the output from starting a session
type StartSessionOutput struct {
	SessionID string `json:"session_id" jsonschema:"the unique session ID"`
	Status    string `json:"status" jsonschema:"the session status (starting)"`
	PID       string `json:"pid,omitempty" jsonschema:"the process ID if available"`
	Message   string `json:"message" jsonschema:"status message"`
}

// StartSessionHandler handles starting a new opencode session
func StartSessionHandler(ctx context.Context, req *mcp.CallToolRequest, input StartSessionInput) (*mcp.CallToolResult, StartSessionOutput, error) {
	if globalSessionManager == nil {
		return nil, StartSessionOutput{}, fmt.Errorf("session manager not initialized")
	}

	session, err := globalSessionManager.CreateSession(
		input.Message,
		input.Title,
		input.SessionID,
		input.Files,
		input.Continue,
		input.Thinking,
	)
	if err != nil {
		return nil, StartSessionOutput{}, err
	}

	output := StartSessionOutput{
		SessionID: session.ID,
		Status:    session.Status,
		PID:       session.PID,
		Message:   "Session started successfully",
	}

	return nil, output, nil
}

// GetSessionStatusInput represents the input for getting session status
type GetSessionStatusInput struct {
	SessionID string `json:"session_id" jsonschema:"the session ID to check"`
}

// GetSessionStatusOutput represents the output from getting session status
type GetSessionStatusOutput struct {
	SessionID string    `json:"session_id" jsonschema:"the session ID"`
	Status    string    `json:"status" jsonschema:"the session status: running, completed, failed, stopped, or not_found"`
	PID       string    `json:"pid,omitempty" jsonschema:"the process ID"`
	ExitCode  string    `json:"exit_code,omitempty" jsonschema:"the exit code if completed"`
	CreatedAt time.Time `json:"created_at" jsonschema:"when the session was created"`
	StartedAt time.Time `json:"started_at,omitempty" jsonschema:"when the session started"`
	StoppedAt time.Time `json:"stopped_at,omitempty" jsonschema:"when the session stopped"`
	Duration  string    `json:"duration,omitempty" jsonschema:"the session duration"`
}

// GetSessionStatusHandler handles getting the status of a session
func GetSessionStatusHandler(ctx context.Context, req *mcp.CallToolRequest, input GetSessionStatusInput) (*mcp.CallToolResult, GetSessionStatusOutput, error) {
	if globalSessionManager == nil {
		return nil, GetSessionStatusOutput{}, fmt.Errorf("session manager not initialized")
	}

	session, exists := globalSessionManager.GetSession(input.SessionID)
	if !exists {
		return nil, GetSessionStatusOutput{
			SessionID: input.SessionID,
			Status:    "not_found",
		}, nil
	}

	output := GetSessionStatusOutput{
		SessionID: session.ID,
		Status:    session.Status,
		PID:       session.PID,
		ExitCode:  session.ExitCode,
		CreatedAt: session.CreatedAt,
		StartedAt: session.StartedAt,
		StoppedAt: session.StoppedAt,
	}

	// Calculate duration
	if !session.StartedAt.IsZero() {
		endTime := session.StoppedAt
		if endTime.IsZero() {
			endTime = time.Now()
		}
		duration := endTime.Sub(session.StartedAt)
		output.Duration = duration.String()
	}

	return nil, output, nil
}

// GetSessionLogsInput represents the input for getting session logs
type GetSessionLogsInput struct {
	SessionID string `json:"session_id" jsonschema:"the session ID"`
	Lines     int    `json:"lines,omitempty" jsonschema:"number of lines to retrieve from the end (default: 100)"`
}

// GetSessionLogsOutput represents the output from getting session logs
type GetSessionLogsOutput struct {
	SessionID  string `json:"session_id" jsonschema:"the session ID"`
	Stdout     string `json:"stdout" jsonschema:"standard output from the session"`
	Stderr     string `json:"stderr" jsonschema:"standard error from the session"`
	LineCount  int    `json:"line_count" jsonschema:"number of lines returned"`
	TotalLines int    `json:"total_lines,omitempty" jsonschema:"total number of lines available"`
}

// GetSessionLogsHandler handles getting logs from a session
func GetSessionLogsHandler(ctx context.Context, req *mcp.CallToolRequest, input GetSessionLogsInput) (*mcp.CallToolResult, GetSessionLogsOutput, error) {
	if globalSessionManager == nil {
		return nil, GetSessionLogsOutput{}, fmt.Errorf("session manager not initialized")
	}

	// Default to 100 lines if not specified
	lines := input.Lines
	if lines <= 0 {
		lines = 100
	}

	stdout, stderr, err := globalSessionManager.GetSessionLogs(input.SessionID, lines)
	if err != nil {
		return nil, GetSessionLogsOutput{}, err
	}

	output := GetSessionLogsOutput{
		SessionID: input.SessionID,
		Stdout:    stdout,
		Stderr:    stderr,
		LineCount: lines,
	}

	return nil, output, nil
}

// StopSessionInput represents the input for stopping a session
type StopSessionInput struct {
	SessionID string `json:"session_id" jsonschema:"the session ID to stop"`
	Force     bool   `json:"force,omitempty" jsonschema:"force kill the process"`
}

// StopSessionOutput represents the output from stopping a session
type StopSessionOutput struct {
	SessionID string `json:"session_id" jsonschema:"the session ID"`
	Status    string `json:"status" jsonschema:"the session status after stopping"`
	Message   string `json:"message" jsonschema:"status message"`
}

// StopSessionHandler handles stopping a session
func StopSessionHandler(ctx context.Context, req *mcp.CallToolRequest, input StopSessionInput) (*mcp.CallToolResult, StopSessionOutput, error) {
	if globalSessionManager == nil {
		return nil, StopSessionOutput{}, fmt.Errorf("session manager not initialized")
	}

	err := globalSessionManager.StopSession(input.SessionID, input.Force)
	if err != nil {
		return nil, StopSessionOutput{}, err
	}

	output := StopSessionOutput{
		SessionID: input.SessionID,
		Status:    "stopped",
		Message:   "Session stopped successfully",
	}

	return nil, output, nil
}

// ListSessionsInput represents the input for listing sessions
type ListSessionsInput struct {
	StatusFilter string `json:"status_filter,omitempty" jsonschema:"filter by status: running, completed, failed, stopped, or all"`
}

// SessionInfo represents a session in the list
type SessionInfo struct {
	ID             string    `json:"id" jsonschema:"the session ID"`
	Status         string    `json:"status" jsonschema:"the session status"`
	PID            string    `json:"pid,omitempty" jsonschema:"the process ID"`
	MessagePreview string    `json:"message_preview" jsonschema:"preview of the message"`
	CreatedAt      time.Time `json:"created_at" jsonschema:"when the session was created"`
	Model          string    `json:"model,omitempty" jsonschema:"the model used"`
}

// ListSessionsOutput represents the output from listing sessions
type ListSessionsOutput struct {
	Sessions []SessionInfo `json:"sessions" jsonschema:"list of sessions"`
	Count    int           `json:"count" jsonschema:"number of sessions"`
}

// ListSessionsHandler handles listing all sessions
func ListSessionsHandler(ctx context.Context, req *mcp.CallToolRequest, input ListSessionsInput) (*mcp.CallToolResult, ListSessionsOutput, error) {
	if globalSessionManager == nil {
		return nil, ListSessionsOutput{}, fmt.Errorf("session manager not initialized")
	}

	sessions := globalSessionManager.ListSessions(input.StatusFilter)

	var sessionInfos []SessionInfo
	for _, session := range sessions {
		// Create message preview (first 100 chars)
		preview := session.Message
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}

		sessionInfos = append(sessionInfos, SessionInfo{
			ID:             session.ID,
			Status:         session.Status,
			PID:            session.PID,
			MessagePreview: preview,
			CreatedAt:      session.CreatedAt,
			Model:          session.Model,
		})
	}

	output := ListSessionsOutput{
		Sessions: sessionInfos,
		Count:    len(sessionInfos),
	}

	return nil, output, nil
}
