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

// CreateSession creates a new session and starts the opencode process
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

	// Get opencode binary path and configuration from environment
	opencodeBinary := getEnv("OPENCODE_BINARY", "opencode")
	model := getEnv("OPENCODE_MODEL", "")
	agent := getEnv("OPENCODE_AGENT", "")
	format := getEnv("OPENCODE_FORMAT", "json")
	share := getEnv("OPENCODE_SHARE", "false")
	attach := getEnv("OPENCODE_ATTACH", "")
	portStr := getEnv("OPENCODE_PORT", "0")
	variant := getEnv("OPENCODE_VARIANT", "")

	port, _ := strconv.Atoi(portStr)

	// Build command arguments
	args := []string{"run"}

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
	for _, file := range files {
		args = append(args, "-f", file)
	}

	// Create process with go-processmanager
	process := processmanager.New(
		processmanager.WithName(opencodeBinary),
		processmanager.WithArgs(args...),
		processmanager.WithStateDir(sessionDir),
		processmanager.WithWorkDir(sessionDir),
	)

	session := &Session{
		ID:        id,
		Status:    "starting",
		Message:   message,
		Model:     getEnv("OPENCODE_MODEL", ""),
		CreatedAt: time.Now(),
		Process:   process,
		StateDir:  sessionDir,
	}

	sm.sessions[id] = session

	// Save state immediately after creation
	if sm.store != nil {
		sm.store.Save(sm.GetAllSessions())
	}

	// Record metrics
	RecordSessionCreated()
	logger := GetLogger()
	logger.Info("Created session %s with PID %s", id, session.PID)

	// Start the process asynchronously
	go sm.runSession(session)

	return session, nil
}

// runSession runs the opencode process and monitors its status
func (sm *SessionManager) runSession(session *Session) {
	session.StartedAt = time.Now()
	session.Status = "running"

	// Set up environment variables for opencode
	env := os.Environ()

	// Add config from environment if provided
	if configContent := os.Getenv("OPENCODE_CONFIG_CONTENT"); configContent != "" {
		env = append(env, fmt.Sprintf("OPENCODE_CONFIG_CONTENT=%s", configContent))
	}
	if configPath := os.Getenv("OPENCODE_CONFIG"); configPath != "" {
		env = append(env, fmt.Sprintf("OPENCODE_CONFIG=%s", configPath))
	}

	// Run the process
	err := session.Process.Run()
	if err != nil {
		sm.updateSessionStatus(session, "failed", "-1")
		return
	}

	session.PID = session.Process.PID

	// Create ticker for periodic state saving
	stateSaveTicker := time.NewTicker(10 * time.Second)
	defer stateSaveTicker.Stop()

	// Wait for process to complete by polling
	// The process manager handles the process lifecycle
	// We poll to check when it's done
	for {
		select {
		case <-stateSaveTicker.C:
			// Periodic save of session state during execution
			if sm.store != nil {
				sm.store.Save(sm.GetAllSessions())
			}
		default:
			time.Sleep(100 * time.Millisecond)
			// Check if process is still running by checking if we can get exit code
			exitCode, err := session.Process.ExitCode()
			if err == nil && exitCode != "" {
				// Process has completed
				sm.updateSessionStatus(session, session.Status, exitCode)
				return
			}
		}
	}
}

// updateSessionStatus safely updates session status with proper locking and persistence
func (sm *SessionManager) updateSessionStatus(session *Session, status string, exitCode string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session.StoppedAt = time.Now()
	session.ExitCode = exitCode
	oldStatus := session.Status

	if exitCode == "0" {
		session.Status = "completed"
	} else {
		session.Status = "failed"
	}

	// Save state immediately
	if sm.store != nil {
		sm.store.Save(sm.GetAllSessions())
	}

	// Log and track metrics
	logger := GetLogger()
	duration := time.Since(session.StartedAt).Round(time.Second)

	if oldStatus == "running" || oldStatus == "starting" {
		if session.Status == "completed" {
			RecordSessionCompleted()
			logger.Info("Session %s completed successfully (duration: %s)", session.ID, duration)
		} else {
			RecordSessionFailed()
			logger.Warn("Session %s failed with exit code %s (duration: %s)", session.ID, exitCode, duration)
		}
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
	session.ExitCode = "-1"

	// Save state immediately
	if sm.store != nil {
		sessions := make(map[string]*Session, len(sm.sessions))
		for sid, s := range sm.sessions {
			sessions[sid] = s
		}
		go sm.store.Save(sessions)
	}

	// Schedule cleanup
	go sm.scheduleCleanup(id)

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

// GetAllSessions returns all sessions as a map (for persistence)
func (sm *SessionManager) GetAllSessions() map[string]*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]*Session, len(sm.sessions))
	for id, session := range sm.sessions {
		result[id] = session
	}
	return result
}

// StopAllSessions stops all running sessions (called on server shutdown)
func (sm *SessionManager) StopAllSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Save final state before stopping
	if sm.store != nil {
		sessions := make(map[string]*Session, len(sm.sessions))
		for sid, s := range sm.sessions {
			sessions[sid] = s
		}
		sm.store.Save(sessions)
		sm.store.StopAutoSave()
	}

	for _, session := range sm.sessions {
		if session.Status == "running" || session.Status == "starting" {
			session.Process.Stop()
			session.Status = "stopped"
			session.StoppedAt = time.Now()
			session.ExitCode = "-1"
		}
	}

	// Save final state after stopping
	if sm.store != nil {
		sessions := make(map[string]*Session, len(sm.sessions))
		for sid, s := range sm.sessions {
			sessions[sid] = s
		}
		sm.store.Save(sessions)
	}
}

// scheduleCleanup schedules cleanup of old session based on retention policy
func (sm *SessionManager) scheduleCleanup(sessionID string) {
	sm.mutex.Lock()
	session, exists := sm.sessions[sessionID]
	if !exists {
		sm.mutex.Unlock()
		return
	}

	// Get retention hours based on session status
	retentionHours := 24
	if sm.retentionHours != nil {
		if hours, ok := sm.retentionHours[session.Status]; ok && hours > 0 {
			retentionHours = hours
		}
	}

	// Also check how old the session is - clean immediately if past retention
	if session.StoppedAt.IsZero() {
		session.StoppedAt = time.Now()
	}

	// Calculate when cleanup should occur
	retentionDuration := time.Duration(retentionHours) * time.Hour
	cleanupTime := session.StoppedAt.Add(retentionDuration)
	timeUntilCleanup := time.Until(cleanupTime)

	// If already past retention, clean up immediately
	if timeUntilCleanup <= 0 {
		sm.mutex.Unlock()
		sm.performCleanup(sessionID)
		return
	}
	sm.mutex.Unlock()

	// Schedule future cleanup
	time.AfterFunc(timeUntilCleanup, func() {
		sm.performCleanup(sessionID)
	})
}

// performCleanup actually performs the cleanup of a session
func (sm *SessionManager) performCleanup(sessionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		// Delete from persistence first
		if sm.store != nil {
			sm.store.Delete(sessionID)
		}

		// Remove session directory
		os.RemoveAll(session.StateDir)

		// Remove from memory
		delete(sm.sessions, sessionID)
	}
}

// getLastNLines returns the last n lines of a string
func getLastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// LoadSessions loads existing sessions from persistence on startup
func (sm *SessionManager) LoadSessions() error {
	if sm.store == nil {
		return nil
	}

	states, err := sm.store.Load()
	if err != nil {
		return err
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for _, state := range states {
		// Check if session directory still exists
		if _, err := os.Stat(state.StateDir); os.IsNotExist(err) {
			// Session directory was deleted, skip this session
			continue
		}

		session := &Session{
			ID:        state.ID,
			Status:    state.Status,
			PID:       state.PID,
			Message:   state.Message,
			Model:     state.Model,
			CreatedAt: state.CreatedAt,
			StartedAt: state.StartedAt,
			StoppedAt: state.StoppedAt,
			ExitCode:  state.ExitCode,
			StateDir:  state.StateDir,
		}

		// Handle recovery based on status
		switch session.Status {
		case "running", "starting":
			// Try to recover process state
			if err := sm.recoverProcess(session); err != nil {
				session.Status = "unknown"
			}
		}

		sm.sessions[session.ID] = session

		// Schedule cleanup based on current status
		if session.Status != "running" && session.Status != "starting" {
			go sm.scheduleCleanup(session.ID)
		}
	}

	return nil
}

// recoverProcess attempts to recover a process from a persisted session
func (sm *SessionManager) recoverProcess(session *Session) error {
	// Check if process is still running by trying to find it
	if session.PID == "" {
		return fmt.Errorf("no PID available for session %s", session.ID)
	}

	// Try to load process state from the session directory
	processStatePath := filepath.Join(session.StateDir, "process.state")
	if _, err := os.Stat(processStatePath); err == nil {
		// Process state file exists, try to recover
		// Note: This is a best-effort recovery
		session.Status = "unknown"
	} else {
		// No process state file, mark as failed
		session.Status = "failed"
		session.ExitCode = "-1"
		session.StoppedAt = time.Now()
	}

	return nil
}
