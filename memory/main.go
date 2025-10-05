package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Memory entry structure
type MemoryEntry struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Memory storage structure
type MemoryStorage struct {
	Entries []MemoryEntry `json:"entries"`
}

// Input types for different operations
type AddMemoryInput struct {
	Content string `json:"content" jsonschema:"the content to store in memory"`
}

type RemoveMemoryInput struct {
	ID string `json:"id" jsonschema:"the ID of the memory entry to remove"`
}

type SearchMemoryInput struct {
	Query string `json:"query" jsonschema:"the search query to find matching memory entries"`
}

// Output types
type AddMemoryOutput struct {
	ID        string    `json:"id" jsonschema:"the ID of the created memory entry"`
	Content   string    `json:"content" jsonschema:"the stored content"`
	CreatedAt time.Time `json:"created_at" jsonschema:"when the entry was created"`
}

type ListMemoryOutput struct {
	Entries []MemoryEntry `json:"entries" jsonschema:"list of all memory entries"`
}

type RemoveMemoryOutput struct {
	Success bool   `json:"success" jsonschema:"whether the removal was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type SearchMemoryOutput struct {
	Query   string        `json:"query" jsonschema:"the search query used"`
	Results []MemoryEntry `json:"results" jsonschema:"matching memory entries"`
	Count   int           `json:"count" jsonschema:"number of matching entries found"`
}

// Global variable to store the memory file path
var memoryFilePath string

// Load memory entries from JSON file
func loadMemory() (*MemoryStorage, error) {
	if _, err := os.Stat(memoryFilePath); os.IsNotExist(err) {
		// File doesn't exist, return empty storage
		return &MemoryStorage{Entries: []MemoryEntry{}}, nil
	}

	data, err := os.ReadFile(memoryFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory file: %w", err)
	}

	var storage MemoryStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to parse memory file: %w", err)
	}

	return &storage, nil
}

// Save memory entries to JSON file
func saveMemory(storage *MemoryStorage) error {
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memory data: %w", err)
	}

	if err := os.WriteFile(memoryFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// Generate a unique ID for memory entries
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Add memory entry
func AddMemory(ctx context.Context, req *mcp.CallToolRequest, input AddMemoryInput) (
	*mcp.CallToolResult,
	AddMemoryOutput,
	error,
) {
	storage, err := loadMemory()
	if err != nil {
		return nil, AddMemoryOutput{}, err
	}

	entry := MemoryEntry{
		ID:        generateID(),
		Content:   input.Content,
		CreatedAt: time.Now(),
	}

	storage.Entries = append(storage.Entries, entry)

	if err := saveMemory(storage); err != nil {
		return nil, AddMemoryOutput{}, err
	}

	output := AddMemoryOutput{
		ID:        entry.ID,
		Content:   entry.Content,
		CreatedAt: entry.CreatedAt,
	}

	return nil, output, nil
}

// List all memory entries
func ListMemory(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (
	*mcp.CallToolResult,
	ListMemoryOutput,
	error,
) {
	storage, err := loadMemory()
	if err != nil {
		return nil, ListMemoryOutput{}, err
	}

	output := ListMemoryOutput{
		Entries: storage.Entries,
	}

	return nil, output, nil
}

// Remove memory entry by ID
func RemoveMemory(ctx context.Context, req *mcp.CallToolRequest, input RemoveMemoryInput) (
	*mcp.CallToolResult,
	RemoveMemoryOutput,
	error,
) {
	storage, err := loadMemory()
	if err != nil {
		return nil, RemoveMemoryOutput{}, err
	}

	// Find and remove the entry
	found := false
	for i, entry := range storage.Entries {
		if entry.ID == input.ID {
			storage.Entries = append(storage.Entries[:i], storage.Entries[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		output := RemoveMemoryOutput{
			Success: false,
			Message: fmt.Sprintf("Memory entry with ID '%s' not found", input.ID),
		}
		return nil, output, nil
	}

	if err := saveMemory(storage); err != nil {
		return nil, RemoveMemoryOutput{}, err
	}

	output := RemoveMemoryOutput{
		Success: true,
		Message: fmt.Sprintf("Memory entry with ID '%s' removed successfully", input.ID),
	}

	return nil, output, nil
}

// Search memory entries by content
func SearchMemory(ctx context.Context, req *mcp.CallToolRequest, input SearchMemoryInput) (
	*mcp.CallToolResult,
	SearchMemoryOutput,
	error,
) {
	storage, err := loadMemory()
	if err != nil {
		return nil, SearchMemoryOutput{}, err
	}

	// Perform case-insensitive search
	query := strings.ToLower(input.Query)
	var results []MemoryEntry

	for _, entry := range storage.Entries {
		if strings.Contains(strings.ToLower(entry.Content), query) {
			results = append(results, entry)
		}
	}

	output := SearchMemoryOutput{
		Query:   input.Query,
		Results: results,
		Count:   len(results),
	}

	return nil, output, nil
}

func main() {
	// Get memory file path from environment variable, default to ./memory.json
	memoryFilePath = os.Getenv("MEMORY_FILE_PATH")
	if memoryFilePath == "" {
		memoryFilePath = "/data/memory.json"
	}

	os.MkdirAll(filepath.Dir(memoryFilePath), 0755)

	// Create a server with memory tools
	server := mcp.NewServer(&mcp.Implementation{Name: "memory", Version: "v1.0.0"}, nil)

	// Register memory tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_memory",
		Description: "Add a new entry to memory storage",
	}, AddMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_memory",
		Description: "List all memory entries",
	}, ListMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_memory",
		Description: "Remove a memory entry by ID",
	}, RemoveMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_memory",
		Description: "Search memory entries by content",
	}, SearchMemory)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
