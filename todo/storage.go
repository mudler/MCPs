package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Storage defines the interface for TODO list persistence
type Storage interface {
	Load() (*TODOList, error)
	Save(*TODOList) error
	WithLock(func() error) error
}

// FileStorage implements Storage using file-based persistence
type FileStorage struct {
	filePath string
}

// NewFileStorage creates a new FileStorage instance
func NewFileStorage(filePath string) *FileStorage {
	return &FileStorage{
		filePath: filePath,
	}
}

// Load loads the TODO list from file
func (fs *FileStorage) Load() (*TODOList, error) {
	list := &TODOList{Items: []TODOItem{}}

	// Check if file exists
	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		// File doesn't exist, return empty list
		return list, nil
	}

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TODO file: %w", err)
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, list); err != nil {
			return nil, fmt.Errorf("failed to parse TODO file: %w", err)
		}
	}

	return list, nil
}

// Save saves the TODO list to file atomically
func (fs *FileStorage) Save(list *TODOList) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(fs.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first
	tempFile := fs.filePath + ".tmp"
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal TODO list: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, fs.filePath); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// WithLock executes a function with file locking
func (fs *FileStorage) WithLock(fn func() error) error {
	lockPath := fs.filePath + ".lock"
	fileLock := flock.New(lockPath)

	// Acquire exclusive lock with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("file is locked by another process")
	}
	defer fileLock.Unlock()

	return fn()
}
