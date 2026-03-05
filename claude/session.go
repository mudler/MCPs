package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	processmanager "github.com/mudler/go-processmanager"
)

// SessionManager methods

// CreateSession creates a new session and starts the Claude process
func (sm *SessionManager) CreateSession(message, title, sessionID string, files []string, useContinue, thinking bool) (*Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check max sessions limit
	if len(sm.sessions) >= sm.maxSessions {
		return nil, fmt.Errorf("maximum number of sessions (%d) reached", sm.maxSessions)
	}

	// Generate unique session ID
	id := uuid.New().String()

	// Create session directory
	sessionDir := filepath.Join(sm.sessionDir, id)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Get Claude binary path and configuration from environment
	claudeBinary := getEnv("CLAUDE_BINARY", "claude")
	model := getEnv("CLAUDE_MODEL", "")
	agent := getEnv("CLAUDE_AGENT", "")
	format := getEnv("CLAUDE_FORMAT", "json")
	share := getEnv("CLAUDE_SHARE", "false")
	attach := getEnv("CLAUDE_ATTACH", "")
	portStr := getEnv("CLAUDE_PORT", "0")
	variant := getEnv("CLAUDE_VARIANT", "")

	port, _ := strconv.Atoi(portStr)

	// Build command arguments
	args := []string{"-p"} // Print mode for non-interactive execution

	// Add message if provided
	if message != "" {
		args = append(args, message)
	}

	// Add optional flags (from environment variables)
	if model != "" {
		args = append(args, "-m", model)
	}
	if agent != "" {
		args = append(args, "--agent", agent)
	}
	if format != "" {
		args = append(args, "--format", format)
	}
	if title != "" {
		args = append(args, "--title", title)
	}
	if sessionID != "" {
		args = append(args, "-s", sessionID)
	}
	if useContinue {
		args = append(args, "-c")
	}
	if share == "true" {
		args = append(args, "--share")
	}
	if attach != "" {
		args = append(args, "--attach", attach)
	}
	if port > 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	if variant != "" {
		args = append(args, "--variant", variant)
	}
	if thinking {
		args = append(args, "--thinking")
	}
	if len(files) > 0 {
		for _, file := range files {
			args = append(args, "-f", file)
		}
	}

	// Add allowed tools if specified
	allowedTools := os.Getenv("CLAUDE_ALLOWED_TOOLS")
	if allowedTools != "" {
		args = append(args, "--allowedTools", allowedTools)
	}

	// Add tools restriction if specified
	tools := os.Getenv("CLAUDE_TOOLS")
	if tools != "" {
		args = append(args, "--tools", tools)
	}

	// Skip permissions if specified (allows all tools without prompting)
	if os.Getenv("CLAUDE_DANGEROUSLY_SKIP_PERMISSIONS") == "true" {
		args = append(args, "--dangerously-skip-permissions")
	}

	// Open /dev/null for child stdin so it doesn't inherit (and steal from)
	// the MCP server's stdin, which would cause EOF errors on the JSON-RPC stream.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", os.DevNull, err)
	}

	// Create process with go-processmanager
	process := processmanager.New(
		processmanager.WithName(claudeBinary),
		processmanager.WithArgs(args...),
		processmanager.WithStateDir(sessionDir),
		processmanager.WithWorkDir(sm.workDir),
		processmanager.WithEnvironment(os.Environ()...),
		processmanager.WithSTDIN(devNull),
	)

	session := &Session{
		ID:        id,
		Status:    "starting",
		Message:   message,
		Model:     getEnv("CLAUDE_MODEL", ""),
		CreatedAt: time.Now(),
		Process:   process,
		StateDir:  sessionDir,
	}

	sm.sessions[id] = session

	// Start the process asynchronously
	go sm.runSession(session)

	return session, nil
}

// runSession runs the Claude process and monitors its status
func (sm *SessionManager) runSession(session *Session) {
	session.StartedAt = time.Now()
	session.Status = "running"

	// Run the process
	err := session.Process.Run()
	if err != nil {
		session.Status = "failed"
		session.ExitCode = "-1"
		session.StoppedAt = time.Now()
		return
	}

	session.PID = session.Process.PID

	// Wait for process to complete by polling
	for {
		time.Sleep(100 * time.Millisecond)
		exitCode, err := session.Process.ExitCode()
		if err == nil && exitCode != "" {
			session.ExitCode = exitCode
			break
		}
	}

	session.StoppedAt = time.Now()

	if session.ExitCode == "0" {
		session.Status = "completed"
	} else {
		session.Status = "failed"
	}

	// Schedule cleanup based on retention policy
	go sm.scheduleCleanup(session.ID)
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	session, exists := sm.sessions[id]
	return session, exists
}

// StopSession stops a running session
func (sm *SessionManager) StopSession(id string, force bool) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	if session.Status != "running" && session.Status != "starting" {
		return fmt.Errorf("session is not running: %s", session.Status)
	}

	if err := session.Process.Stop(); err != nil {
		return err
	}

	session.Status = "stopped"
	session.StoppedAt = time.Now()
	return nil
}

// GetSessionLogs retrieves stdout and stderr logs from a session
func (sm *SessionManager) GetSessionLogs(id string, lines int) (stdout, stderr string, err error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[id]
	if !exists {
		return "", "", fmt.Errorf("session not found: %s", id)
	}

	stdoutPath := session.Process.StdoutPath()
	stderrPath := session.Process.StderrPath()

	stdoutBytes, err := os.ReadFile(stdoutPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("failed to read stdout: %w", err)
	}

	stderrBytes, err := os.ReadFile(stderrPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("failed to read stderr: %w", err)
	}

	stdout = string(stdoutBytes)
	stderr = string(stderrBytes)

	// Limit to specified number of lines (from end)
	if lines > 0 {
		stdout = getLastNLines(stdout, lines)
		stderr = getLastNLines(stderr, lines)
	}

	return stdout, stderr, nil
}

// ListSessions returns all sessions, optionally filtered by status
func (sm *SessionManager) ListSessions(statusFilter string) []*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var result []*Session
	for _, session := range sm.sessions {
		if statusFilter == "" || statusFilter == "all" || session.Status == statusFilter {
			result = append(result, session)
		}
	}
	return result
}

// StopAllSessions stops all running sessions (called on server shutdown)
func (sm *SessionManager) StopAllSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for _, session := range sm.sessions {
		if session.Status == "running" || session.Status == "starting" {
			session.Process.Stop()
		}
		// Clean up session directory
		os.RemoveAll(session.StateDir)
	}
	// Clear sessions map
	sm.sessions = make(map[string]*Session)
}

// scheduleCleanup schedules cleanup of old session based on retention policy
func (sm *SessionManager) scheduleCleanup(sessionID string) {
	retentionHours := getEnvInt("CLAUDE_LOG_RETENTION_HOURS", 24)
	if retentionHours <= 0 {
		return // No cleanup scheduled
	}

	time.AfterFunc(time.Duration(retentionHours)*time.Hour, func() {
		sm.mutex.Lock()
		defer sm.mutex.Unlock()

		if session, exists := sm.sessions[sessionID]; exists {
			os.RemoveAll(session.StateDir)
			delete(sm.sessions, sessionID)
		}
	})
}

// getLastNLines returns the last n lines of a string
func getLastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
