package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Memory entry structure
type MemoryEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Input types for different operations
type AddMemoryInput struct {
	Name    string `json:"name" jsonschema:"the name/title of the memory entry"`
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
	Name      string    `json:"name" jsonschema:"the name of the memory entry"`
	Content   string    `json:"content" jsonschema:"the stored content"`
	CreatedAt time.Time `json:"created_at" jsonschema:"when the entry was created"`
}

type ListMemoryOutput struct {
	Names []string `json:"names" jsonschema:"list of memory entry names"`
	Count int      `json:"count" jsonschema:"number of entries"`
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

// Global variable to store the bleve index
var index bleve.Index
var indexPath string

// Generate a unique ID for memory entries
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Initialize bleve index
func initBleveIndex() error {
	// Check if index already exists
	if _, err := os.Stat(indexPath); err == nil {
		// Index exists, open it
		var err error
		index, err = bleve.Open(indexPath)
		if err != nil {
			return fmt.Errorf("failed to open bleve index: %w", err)
		}
		return nil
	}

	// Create new index mapping
	mapping := bleve.NewIndexMapping()

	// Create document mapping for memory entries
	entryMapping := bleve.NewDocumentMapping()

	// Map name field as text (searchable and stored)
	nameFieldMapping := bleve.NewTextFieldMapping()
	nameFieldMapping.Analyzer = "standard"
	nameFieldMapping.Store = true
	entryMapping.AddFieldMappingsAt("name", nameFieldMapping)

	// Map content field as text (searchable and stored)
	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = "standard"
	contentFieldMapping.Store = true
	entryMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// Map created_at field as datetime (stored but not indexed for search)
	dateFieldMapping := bleve.NewDateTimeFieldMapping()
	dateFieldMapping.Store = true
	entryMapping.AddFieldMappingsAt("created_at", dateFieldMapping)

	// Add document mapping to index mapping
	mapping.AddDocumentMapping("_default", entryMapping)

	// Create the index
	var err error
	index, err = bleve.New(indexPath, mapping)
	if err != nil {
		return fmt.Errorf("failed to create bleve index: %w", err)
	}

	return nil
}

// Add memory entry
func AddMemory(ctx context.Context, req *mcp.CallToolRequest, input AddMemoryInput) (
	*mcp.CallToolResult,
	AddMemoryOutput,
	error,
) {
	entry := MemoryEntry{
		ID:        generateID(),
		Name:      input.Name,
		Content:   input.Content,
		CreatedAt: time.Now(),
	}

	// Index the entry in bleve
	if err := index.Index(entry.ID, entry); err != nil {
		return nil, AddMemoryOutput{}, fmt.Errorf("failed to index memory entry: %w", err)
	}

	output := AddMemoryOutput{
		ID:        entry.ID,
		Name:      entry.Name,
		Content:   entry.Content,
		CreatedAt: entry.CreatedAt,
	}

	return nil, output, nil
}

// List all memory entries (returns only names)
func ListMemory(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (
	*mcp.CallToolResult,
	ListMemoryOutput,
	error,
) {
	// Create a match all query to get all documents
	query := bleve.NewMatchAllQuery()
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = 10000              // Get up to 10000 entries
	searchRequest.Fields = []string{"name"} // Only fetch the name field

	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, ListMemoryOutput{}, fmt.Errorf("failed to search index: %w", err)
	}

	names := make([]string, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		// Extract name from stored fields
		if nameVal, ok := hit.Fields["name"].(string); ok {
			names = append(names, nameVal)
		}
	}

	output := ListMemoryOutput{
		Names: names,
		Count: len(names),
	}

	return nil, output, nil
}

// Remove memory entry by ID
func RemoveMemory(ctx context.Context, req *mcp.CallToolRequest, input RemoveMemoryInput) (
	*mcp.CallToolResult,
	RemoveMemoryOutput,
	error,
) {
	// Check if document exists by trying to get it
	doc, err := index.Document(input.ID)
	if err != nil {
		return nil, RemoveMemoryOutput{}, fmt.Errorf("failed to check document existence: %w", err)
	}

	if doc == nil {
		output := RemoveMemoryOutput{
			Success: false,
			Message: fmt.Sprintf("Memory entry with ID '%s' not found", input.ID),
		}
		return nil, output, nil
	}

	// Delete the document from the index
	if err := index.Delete(input.ID); err != nil {
		return nil, RemoveMemoryOutput{}, fmt.Errorf("failed to delete memory entry: %w", err)
	}

	output := RemoveMemoryOutput{
		Success: true,
		Message: fmt.Sprintf("Memory entry with ID '%s' removed successfully", input.ID),
	}

	return nil, output, nil
}

// Search memory entries by name and content
func SearchMemory(ctx context.Context, req *mcp.CallToolRequest, input SearchMemoryInput) (
	*mcp.CallToolResult,
	SearchMemoryOutput,
	error,
) {
	// Use disjunction query to search both name and content fields
	// This is more flexible and handles multi-word queries better
	nameQuery := bleve.NewMatchQuery(input.Query)
	nameQuery.SetField("name")
	contentQuery := bleve.NewMatchQuery(input.Query)
	contentQuery.SetField("content")
	disjunctionQuery := bleve.NewDisjunctionQuery(nameQuery, contentQuery)

	searchRequest := bleve.NewSearchRequest(disjunctionQuery)
	searchRequest.Size = 100                                         // Limit results to 100
	searchRequest.Fields = []string{"name", "content", "created_at"} // Request stored fields

	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, SearchMemoryOutput{}, fmt.Errorf("failed to search index: %w", err)
	}

	results := make([]MemoryEntry, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		entry := MemoryEntry{
			ID: hit.ID,
		}

		// Try to extract fields from stored fields in search result first
		if nameVal, ok := hit.Fields["name"].(string); ok {
			entry.Name = nameVal
		}
		if contentVal, ok := hit.Fields["content"].(string); ok {
			entry.Content = contentVal
		}
		if createdAtVal, ok := hit.Fields["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdAtVal); err == nil {
				entry.CreatedAt = t
			}
		} else if createdAtVal, ok := hit.Fields["created_at"].(time.Time); ok {
			entry.CreatedAt = createdAtVal
		}

		// If fields are missing, try to fetch document and reconstruct from stored data
		// Note: This is a fallback - stored fields should work with Store = true
		if entry.Name == "" || entry.Content == "" {
			doc, err := index.Document(hit.ID)
			if err == nil && doc != nil {
				// Iterate through document to find stored field values
				// Bleve stores fields, but we need to access them via the document API
				// For now, log a warning and skip if fields aren't in stored fields
				log.Printf("Warning: Missing stored fields for document %s, stored fields should be used", hit.ID)
			}
		}

		results = append(results, entry)
	}

	output := SearchMemoryOutput{
		Query:   input.Query,
		Results: results,
		Count:   len(results),
	}

	return nil, output, nil
}

func main() {
	// Get index path from environment variable, default to /data/memory.bleve
	indexPath = os.Getenv("MEMORY_INDEX_PATH")
	if indexPath == "" {
		indexPath = "/data/memory.bleve"
	}

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(indexPath), 0755)

	// Initialize bleve index
	if err := initBleveIndex(); err != nil {
		log.Fatalf("Failed to initialize bleve index: %v", err)
	}

	// Create a server with memory tools
	server := mcp.NewServer(&mcp.Implementation{Name: "memory", Version: "v2.0.0"}, nil)

	// Register memory tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_memory",
		Description: "Add a new entry to memory storage",
	}, AddMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_memory",
		Description: "List all memory entry names",
	}, ListMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_memory",
		Description: "Remove a memory entry by ID",
	}, RemoveMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_memory",
		Description: "Search memory entries by name and content using full-text search",
	}, SearchMemory)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
