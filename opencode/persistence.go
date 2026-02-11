package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionState represents a serializable version of session data for persistence
type SessionState struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	PID         string    `json:"pid"`
	Message     string    `json:"message"`
	Model       string    `json:"model"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	StoppedAt   time.Time `json:"stopped_at,omitempty"`
	ExitCode    string    `json:"exit_code"`
	StateDir    string    `json:"state_dir"`
	LastSaved   time.Time `json:"last_saved"`
	IsRecovered bool      `json:"is_recovered"`
}

// SessionStore defines the interface for session persistence
type SessionStore interface {
	Save(sessions map[string]*Session) error
	Load() ([]SessionState, error)
	Delete(sessionID string) error
	Backup() error
}

// JSONSessionStore implements SessionStore using JSON files
type JSONSessionStore struct {
	sessionDir      string
	sessionsFile    string
	backupDir       string
	autoSaveEnabled bool
	autoSaveTicker  *time.Ticker
	stopChan        chan bool
	mutex           sync.RWMutex
	sessionManager  *SessionManager
}

// NewJSONSessionStore creates a new JSON-based session store
func NewJSONSessionStore(sessionDir string, sessionManager *SessionManager) *JSONSessionStore {
	store := &JSONSessionStore{
		sessionDir:      sessionDir,
		sessionsFile:    filepath.Join(sessionDir, "sessions.json"),
		backupDir:       filepath.Join(sessionDir, "backups"),
		autoSaveEnabled: getEnvBool("OPENCODE_PERSISTENCE_ENABLED", true),
		stopChan:        make(chan bool),
		sessionManager:  sessionManager,
	}

	// Ensure directories exist
	os.MkdirAll(sessionDir, 0755)
	os.MkdirAll(store.backupDir, 0755)

	return store
}

// StartAutoSave begins periodic auto-saving of session state
func (js *JSONSessionStore) StartAutoSave(interval time.Duration) {
	if !js.autoSaveEnabled {
		return
	}

	js.autoSaveTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-js.autoSaveTicker.C:
				if js.sessionManager != nil {
					sessions := js.sessionManager.GetAllSessions()
					if err := js.Save(sessions); err != nil {
						// Log error but don't crash
						logger := GetLogger()
						logger.Warn("Failed to auto-save sessions: %v", err)
					}
				}
			case <-js.stopChan:
				return
			}
		}
	}()
}

// StopAutoSave stops the auto-save goroutine
func (js *JSONSessionStore) StopAutoSave() {
	js.mutex.Lock()
	defer js.mutex.Unlock()
	if js.autoSaveTicker != nil {
		js.autoSaveTicker.Stop()
		select {
		case <-js.stopChan:
			// Already closed, do nothing
		default:
			close(js.stopChan)
		}
		js.autoSaveTicker = nil
	}
}

// Save persists all sessions to JSON file atomically
func (js *JSONSessionStore) Save(sessions map[string]*Session) error {
	if !js.autoSaveEnabled {
		return nil
	}

	js.mutex.Lock()
	defer js.mutex.Unlock()

	// Convert sessions to serializable state
	states := make([]SessionState, 0, len(sessions))
	for _, session := range sessions {
		states = append(states, SessionState{
			ID:          session.ID,
			Status:      session.Status,
			PID:         session.PID,
			Message:     session.Message,
			Model:       session.Model,
			CreatedAt:   session.CreatedAt,
			StartedAt:   session.StartedAt,
			StoppedAt:   session.StoppedAt,
			ExitCode:    session.ExitCode,
			StateDir:    session.StateDir,
			LastSaved:   time.Now(),
			IsRecovered: session.Status == "running" || session.Status == "starting",
		})
	}

	// Create backup of current file if it exists
	if _, err := os.Stat(js.sessionsFile); err == nil {
		backupFile := filepath.Join(js.backupDir, fmt.Sprintf("sessions_%s.json", time.Now().Format("20060102_150405")))
		os.Rename(js.sessionsFile, backupFile)

		// Clean up old backups (keep only last 10)
		js.cleanupOldBackups()
	}

	// Write to temp file first (atomic write)
	tempFile := js.sessionsFile + ".tmp"
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, js.sessionsFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load retrieves all persisted session states
func (js *JSONSessionStore) Load() ([]SessionState, error) {
	js.mutex.RLock()
	defer js.mutex.RUnlock()

	// Check if file exists
	if _, err := os.Stat(js.sessionsFile); os.IsNotExist(err) {
		return []SessionState{}, nil // No sessions yet
	}

	data, err := os.ReadFile(js.sessionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions file: %w", err)
	}

	var states []SessionState
	if err := json.Unmarshal(data, &states); err != nil {
		// Try to recover from backup
		states, err = js.loadFromBackup()
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal sessions and backup also failed: %w", err)
		}
	}

	return states, nil
}

// loadFromBackup attempts to load from the most recent backup
func (js *JSONSessionStore) loadFromBackup() ([]SessionState, error) {
	entries, err := os.ReadDir(js.backupDir)
	if err != nil {
		return nil, err
	}

	// Find most recent backup
	var mostRecent os.DirEntry
	var mostRecentTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(mostRecentTime) {
				mostRecent = entry
				mostRecentTime = info.ModTime()
			}
		}
	}

	if mostRecent == nil {
		return nil, fmt.Errorf("no backup files found")
	}

	backupFile := filepath.Join(js.backupDir, mostRecent.Name())
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return nil, err
	}

	var states []SessionState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, err
	}

	return states, nil
}

// Delete removes a session from persistence
func (js *JSONSessionStore) Delete(sessionID string) error {
	js.mutex.Lock()
	defer js.mutex.Unlock()

	// Reload, remove session, and save
	states, err := js.loadWithoutLock()
	if err != nil {
		return err
	}

	// Filter out the deleted session
	filtered := make([]SessionState, 0, len(states))
	for _, state := range states {
		if state.ID != sessionID {
			filtered = append(filtered, state)
		}
	}

	// Save back
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(js.sessionsFile, data, 0644)
}

// Backup creates an explicit backup of current sessions
func (js *JSONSessionStore) Backup() error {
	sessions := js.sessionManager.GetAllSessions()
	return js.Save(sessions)
}

// loadWithoutLock loads sessions without acquiring lock (caller must hold lock)
func (js *JSONSessionStore) loadWithoutLock() ([]SessionState, error) {
	if _, err := os.Stat(js.sessionsFile); os.IsNotExist(err) {
		return []SessionState{}, nil
	}

	data, err := os.ReadFile(js.sessionsFile)
	if err != nil {
		return nil, err
	}

	var states []SessionState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, err
	}

	return states, nil
}

// cleanupOldBackups removes old backup files, keeping only the last 10
func (js *JSONSessionStore) cleanupOldBackups() {
	entries, err := os.ReadDir(js.backupDir)
	if err != nil {
		return
	}

	// Collect all backup files with their mod times
	type backupInfo struct {
		path string
		time time.Time
	}
	backups := []backupInfo{}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			backups = append(backups, backupInfo{
				path: filepath.Join(js.backupDir, entry.Name()),
				time: info.ModTime(),
			})
		}
	}

	// Sort by time (newest first) and remove old ones
	if len(backups) > 10 {
		// Simple bubble sort by time
		for i := 0; i < len(backups); i++ {
			for j := i + 1; j < len(backups); j++ {
				if backups[j].time.After(backups[i].time) {
					backups[i], backups[j] = backups[j], backups[i]
				}
			}
		}

		// Remove old backups
		for i := 10; i < len(backups); i++ {
			os.Remove(backups[i].path)
		}
	}
}

// Helper function to get boolean from environment
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if value == "true" || value == "1" || value == "yes" {
			return true
		}
		if value == "false" || value == "0" || value == "no" {
			return false
		}
	}
	return defaultValue
}

// Helper function to get duration from environment
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return defaultValue
}
