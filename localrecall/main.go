package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Global HTTP client for LocalRecall API
var httpClient *http.Client
var localRecallURL string
var apiKey string
var defaultCollectionName string

// LocalRecall API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Input types for tools
type SearchInput struct {
	CollectionName string `json:"collection_name" jsonschema:"the name of the collection to search"`
	Query          string `json:"query" jsonschema:"the search query"`
	MaxResults     int    `json:"max_results,omitempty" jsonschema:"maximum number of results to return (default: 5)"`
}

type SearchInputWithoutCollection struct {
	Query      string `json:"query" jsonschema:"the search query"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"maximum number of results to return (default: 5)"`
}

type CreateCollectionInput struct {
	Name string `json:"name" jsonschema:"the name of the collection to create"`
}

type ResetCollectionInput struct {
	Name string `json:"name" jsonschema:"the name of the collection to reset"`
}

type AddDocumentInput struct {
	CollectionName string `json:"collection_name" jsonschema:"the name of the collection"`
	FilePath       string `json:"file_path,omitempty" jsonschema:"path to the file to upload (mutually exclusive with file_content)"`
	FileContent    string `json:"file_content,omitempty" jsonschema:"file content as string (mutually exclusive with file_path)"`
	Filename       string `json:"filename" jsonschema:"the filename for the document"`
}

type AddDocumentInputWithoutCollection struct {
	FilePath    string `json:"file_path,omitempty" jsonschema:"path to the file to upload (mutually exclusive with file_content)"`
	FileContent string `json:"file_content,omitempty" jsonschema:"file content as string (mutually exclusive with file_path)"`
	Filename    string `json:"filename" jsonschema:"the filename for the document"`
}

type ListFilesInput struct {
	CollectionName string `json:"collection_name" jsonschema:"the name of the collection"`
}

type ListFilesInputWithoutCollection struct {
}

type DeleteEntryInput struct {
	CollectionName string `json:"collection_name" jsonschema:"the name of the collection"`
	Entry          string `json:"entry" jsonschema:"the filename of the entry to delete"`
}

type DeleteEntryInputWithoutCollection struct {
	Entry string `json:"entry" jsonschema:"the filename of the entry to delete"`
}

// Output types for tools
type SearchOutput struct {
	Query      string                   `json:"query" jsonschema:"the search query"`
	MaxResults int                      `json:"max_results" jsonschema:"maximum number of results requested"`
	Results    []map[string]interface{} `json:"results" jsonschema:"search results"`
	Count      int                      `json:"count" jsonschema:"number of results returned"`
}

type CreateCollectionOutput struct {
	Name      string `json:"name" jsonschema:"the name of the collection"`
	CreatedAt string `json:"created_at" jsonschema:"timestamp when the collection was created"`
}

type ResetCollectionOutput struct {
	Collection string `json:"collection" jsonschema:"the name of the collection"`
	ResetAt    string `json:"reset_at" jsonschema:"timestamp when the collection was reset"`
}

type AddDocumentOutput struct {
	Filename   string `json:"filename" jsonschema:"the filename of the uploaded document"`
	Collection string `json:"collection" jsonschema:"the name of the collection"`
	UploadedAt string `json:"uploaded_at" jsonschema:"timestamp when the document was uploaded"`
}

type ListCollectionsOutput struct {
	Collections []string `json:"collections" jsonschema:"list of collection names"`
	Count       int      `json:"count" jsonschema:"number of collections"`
}

type ListFilesOutput struct {
	Collection string   `json:"collection" jsonschema:"the name of the collection"`
	Entries    []string `json:"entries" jsonschema:"list of entry filenames"`
	Count      int      `json:"count" jsonschema:"number of entries"`
}

type DeleteEntryOutput struct {
	DeletedEntry    string   `json:"deleted_entry" jsonschema:"the filename of the deleted entry"`
	RemainingEntries []string `json:"remaining_entries" jsonschema:"list of remaining entry filenames"`
	EntryCount      int      `json:"entry_count" jsonschema:"number of remaining entries"`
}

// makeRequest makes an HTTP request to the LocalRecall API
func makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*APIResponse, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, localRecallURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		errorMsg := "unknown error"
		if apiResp.Error != nil {
			errorMsg = fmt.Sprintf("%s: %s", apiResp.Error.Code, apiResp.Error.Message)
			if apiResp.Error.Details != "" {
				errorMsg += " - " + apiResp.Error.Details
			}
		}
		return nil, fmt.Errorf("API error: %s", errorMsg)
	}

	return &apiResp, nil
}

// makeMultipartRequest makes a multipart form request for file uploads
func makeMultipartRequest(ctx context.Context, endpoint, filename string, fileContent []byte) (*APIResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(fileContent); err != nil {
		return nil, fmt.Errorf("failed to write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", localRecallURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !apiResp.Success {
		errorMsg := "unknown error"
		if apiResp.Error != nil {
			errorMsg = fmt.Sprintf("%s: %s", apiResp.Error.Code, apiResp.Error.Message)
			if apiResp.Error.Details != "" {
				errorMsg += " - " + apiResp.Error.Details
			}
		}
		return nil, fmt.Errorf("API error: %s", errorMsg)
	}

	return &apiResp, nil
}

// Search searches content in a LocalRecall collection
func Search(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (
	*mcp.CallToolResult,
	SearchOutput,
	error,
) {
	return searchWithCollection(ctx, input.CollectionName, input.Query, input.MaxResults)
}

// SearchWithoutCollection searches content in a LocalRecall collection using default collection
func SearchWithoutCollection(ctx context.Context, req *mcp.CallToolRequest, input SearchInputWithoutCollection) (
	*mcp.CallToolResult,
	SearchOutput,
	error,
) {
	return searchWithCollection(ctx, defaultCollectionName, input.Query, input.MaxResults)
}

// searchWithCollection is the internal implementation for search
func searchWithCollection(ctx context.Context, collectionName, query string, maxResults int) (
	*mcp.CallToolResult,
	SearchOutput,
	error,
) {

	if maxResults == 0 {
		maxResults = 5
	}

	requestBody := map[string]interface{}{
		"query":       query,
		"max_results": maxResults,
	}

	apiResp, err := makeRequest(ctx, "POST", fmt.Sprintf("/api/collections/%s/search", collectionName), requestBody)
	if err != nil {
		return nil, SearchOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, SearchOutput{}, fmt.Errorf("unexpected response data format")
	}

	results := []map[string]interface{}{}
	if resultsData, ok := data["results"].([]interface{}); ok {
		for _, r := range resultsData {
			if resultMap, ok := r.(map[string]interface{}); ok {
				results = append(results, resultMap)
			}
		}
	}

	count := 0
	if countVal, ok := data["count"].(float64); ok {
		count = int(countVal)
	}

	output := SearchOutput{
		Query:      query,
		MaxResults: maxResults,
		Results:    results,
		Count:      count,
	}

	return nil, output, nil
}

// CreateCollection creates a new collection
func CreateCollection(ctx context.Context, req *mcp.CallToolRequest, input CreateCollectionInput) (
	*mcp.CallToolResult,
	CreateCollectionOutput,
	error,
) {
	requestBody := map[string]interface{}{
		"name": input.Name,
	}

	apiResp, err := makeRequest(ctx, "POST", "/api/collections", requestBody)
	if err != nil {
		return nil, CreateCollectionOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, CreateCollectionOutput{}, fmt.Errorf("unexpected response data format")
	}

	createdAt := ""
	if createdAtVal, ok := data["created_at"].(string); ok {
		createdAt = createdAtVal
	}

	output := CreateCollectionOutput{
		Name:      input.Name,
		CreatedAt: createdAt,
	}

	return nil, output, nil
}

// ResetCollection resets (clears) a collection
func ResetCollection(ctx context.Context, req *mcp.CallToolRequest, input ResetCollectionInput) (
	*mcp.CallToolResult,
	ResetCollectionOutput,
	error,
) {
	apiResp, err := makeRequest(ctx, "POST", fmt.Sprintf("/api/collections/%s/reset", input.Name), nil)
	if err != nil {
		return nil, ResetCollectionOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, ResetCollectionOutput{}, fmt.Errorf("unexpected response data format")
	}

	resetAt := ""
	if resetAtVal, ok := data["reset_at"].(string); ok {
		resetAt = resetAtVal
	}

	output := ResetCollectionOutput{
		Collection: input.Name,
		ResetAt:    resetAt,
	}

	return nil, output, nil
}

// AddDocument adds a document to a collection
func AddDocument(ctx context.Context, req *mcp.CallToolRequest, input AddDocumentInput) (
	*mcp.CallToolResult,
	AddDocumentOutput,
	error,
) {
	return addDocumentWithCollection(ctx, input.CollectionName, input.FilePath, input.FileContent, input.Filename)
}

// AddDocumentWithoutCollection adds a document to a collection using default collection
func AddDocumentWithoutCollection(ctx context.Context, req *mcp.CallToolRequest, input AddDocumentInputWithoutCollection) (
	*mcp.CallToolResult,
	AddDocumentOutput,
	error,
) {
	return addDocumentWithCollection(ctx, defaultCollectionName, input.FilePath, input.FileContent, input.Filename)
}

// addDocumentWithCollection is the internal implementation for add document
func addDocumentWithCollection(ctx context.Context, collectionName, filePath, fileContent, filename string) (
	*mcp.CallToolResult,
	AddDocumentOutput,
	error,
) {
	var fileContentBytes []byte
	var err error

	if filePath != "" && fileContent != "" {
		return nil, AddDocumentOutput{}, fmt.Errorf("cannot specify both file_path and file_content")
	}

	if filePath != "" {
		fileContentBytes, err = os.ReadFile(filePath)
		if err != nil {
			return nil, AddDocumentOutput{}, fmt.Errorf("failed to read file: %w", err)
		}
	} else if fileContent != "" {
		fileContentBytes = []byte(fileContent)
	} else {
		return nil, AddDocumentOutput{}, fmt.Errorf("must specify either file_path or file_content")
	}

	apiResp, err := makeMultipartRequest(ctx, fmt.Sprintf("/api/collections/%s/upload", collectionName), filename, fileContentBytes)
	if err != nil {
		return nil, AddDocumentOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, AddDocumentOutput{}, fmt.Errorf("unexpected response data format")
	}

	uploadedAt := ""
	if uploadedAtVal, ok := data["uploaded_at"].(string); ok {
		uploadedAt = uploadedAtVal
	}

	output := AddDocumentOutput{
		Filename:   filename,
		Collection: collectionName,
		UploadedAt: uploadedAt,
	}

	return nil, output, nil
}

// ListCollections lists all collections
func ListCollections(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (
	*mcp.CallToolResult,
	ListCollectionsOutput,
	error,
) {
	apiResp, err := makeRequest(ctx, "GET", "/api/collections", nil)
	if err != nil {
		return nil, ListCollectionsOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, ListCollectionsOutput{}, fmt.Errorf("unexpected response data format")
	}

	collections := []string{}
	if collectionsData, ok := data["collections"].([]interface{}); ok {
		for _, c := range collectionsData {
			if col, ok := c.(string); ok {
				collections = append(collections, col)
			}
		}
	}

	count := 0
	if countVal, ok := data["count"].(float64); ok {
		count = int(countVal)
	}

	output := ListCollectionsOutput{
		Collections: collections,
		Count:       count,
	}

	return nil, output, nil
}

// ListFiles lists files in a collection
func ListFiles(ctx context.Context, req *mcp.CallToolRequest, input ListFilesInput) (
	*mcp.CallToolResult,
	ListFilesOutput,
	error,
) {
	return listFilesWithCollection(ctx, input.CollectionName)
}

// ListFilesWithoutCollection lists files in a collection using default collection
func ListFilesWithoutCollection(ctx context.Context, req *mcp.CallToolRequest, input ListFilesInputWithoutCollection) (
	*mcp.CallToolResult,
	ListFilesOutput,
	error,
) {
	return listFilesWithCollection(ctx, defaultCollectionName)
}

// listFilesWithCollection is the internal implementation for list files
func listFilesWithCollection(ctx context.Context, collectionName string) (
	*mcp.CallToolResult,
	ListFilesOutput,
	error,
) {
	apiResp, err := makeRequest(ctx, "GET", fmt.Sprintf("/api/collections/%s/entries", collectionName), nil)
	if err != nil {
		return nil, ListFilesOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, ListFilesOutput{}, fmt.Errorf("unexpected response data format")
	}

	entries := []string{}
	if entriesData, ok := data["entries"].([]interface{}); ok {
		for _, e := range entriesData {
			if entry, ok := e.(string); ok {
				entries = append(entries, entry)
			}
		}
	}

	count := 0
	if countVal, ok := data["count"].(float64); ok {
		count = int(countVal)
	}

	output := ListFilesOutput{
		Collection: collectionName,
		Entries:    entries,
		Count:      count,
	}

	return nil, output, nil
}

// DeleteEntry deletes an entry from a collection
func DeleteEntry(ctx context.Context, req *mcp.CallToolRequest, input DeleteEntryInput) (
	*mcp.CallToolResult,
	DeleteEntryOutput,
	error,
) {
	return deleteEntryWithCollection(ctx, input.CollectionName, input.Entry)
}

// DeleteEntryWithoutCollection deletes an entry from a collection using default collection
func DeleteEntryWithoutCollection(ctx context.Context, req *mcp.CallToolRequest, input DeleteEntryInputWithoutCollection) (
	*mcp.CallToolResult,
	DeleteEntryOutput,
	error,
) {
	return deleteEntryWithCollection(ctx, defaultCollectionName, input.Entry)
}

// deleteEntryWithCollection is the internal implementation for delete entry
func deleteEntryWithCollection(ctx context.Context, collectionName, entry string) (
	*mcp.CallToolResult,
	DeleteEntryOutput,
	error,
) {
	requestBody := map[string]interface{}{
		"entry": entry,
	}

	apiResp, err := makeRequest(ctx, "DELETE", fmt.Sprintf("/api/collections/%s/entry/delete", collectionName), requestBody)
	if err != nil {
		return nil, DeleteEntryOutput{}, err
	}

	// Extract data from response
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		return nil, DeleteEntryOutput{}, fmt.Errorf("unexpected response data format")
	}

	remainingEntries := []string{}
	entryCount := 0

	if remainingData, ok := data["remaining_entries"].([]interface{}); ok {
		for _, e := range remainingData {
			if entryStr, ok := e.(string); ok {
				remainingEntries = append(remainingEntries, entryStr)
			}
		}
		entryCount = len(remainingEntries)
	}

	if countVal, ok := data["entry_count"].(float64); ok {
		entryCount = int(countVal)
	}

	output := DeleteEntryOutput{
		DeletedEntry:     entry,
		RemainingEntries: remainingEntries,
		EntryCount:       entryCount,
	}

	return nil, output, nil
}

func main() {
	// Get configuration from environment variables
	localRecallURL = os.Getenv("LOCALRECALL_URL")
	if localRecallURL == "" {
		localRecallURL = "http://localhost:8080"
	}

	apiKey = os.Getenv("LOCALRECALL_API_KEY")
	defaultCollectionName = os.Getenv("LOCALRECALL_COLLECTION")

	// Create HTTP client with timeout
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Parse enabled tools
	enabledToolsStr := os.Getenv("LOCALRECALL_ENABLED_TOOLS")
	enabledTools := make(map[string]bool)

	// Valid tool names
	validTools := map[string]bool{
		"search":           true,
		"create_collection": true,
		"reset_collection": true,
		"add_document":     true,
		"list_collections": true,
		"list_files":       true,
		"delete_entry":     true,
	}

	if enabledToolsStr != "" {
		// Parse comma-separated list
		tools := strings.Split(enabledToolsStr, ",")
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			if tool == "" {
				continue
			}
			if validTools[tool] {
				enabledTools[tool] = true
			} else {
				log.Printf("Warning: Unknown tool name '%s' will be ignored", tool)
			}
		}
	} else {
		// If not specified, enable all tools
		for tool := range validTools {
			enabledTools[tool] = true
		}
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "localrecall",
		Version: "v1.0.0",
	}, nil)

	// Register tools based on enabled list
	if enabledTools["search"] {
		if defaultCollectionName != "" {
			desc := fmt.Sprintf("Search content in LocalRecall collection '%s'", defaultCollectionName)
			mcp.AddTool(server, &mcp.Tool{
				Name:        "search",
				Description: desc,
			}, SearchWithoutCollection)
			log.Printf("Tool 'search' enabled (using default collection: %s)", defaultCollectionName)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "search",
				Description: "Search content in a LocalRecall collection",
			}, Search)
			log.Println("Tool 'search' enabled")
		}
	}

	if enabledTools["create_collection"] {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "create_collection",
			Description: "Create a new collection in LocalRecall",
		}, CreateCollection)
		log.Println("Tool 'create_collection' enabled")
	}

	if enabledTools["reset_collection"] {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "reset_collection",
			Description: "Reset (clear) a collection in LocalRecall",
		}, ResetCollection)
		log.Println("Tool 'reset_collection' enabled")
	}

	if enabledTools["add_document"] {
		if defaultCollectionName != "" {
			desc := fmt.Sprintf("Add a document to LocalRecall collection '%s'", defaultCollectionName)
			mcp.AddTool(server, &mcp.Tool{
				Name:        "add_document",
				Description: desc,
			}, AddDocumentWithoutCollection)
			log.Printf("Tool 'add_document' enabled (using default collection: %s)", defaultCollectionName)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "add_document",
				Description: "Add a document to a LocalRecall collection",
			}, AddDocument)
			log.Println("Tool 'add_document' enabled")
		}
	}

	if enabledTools["list_collections"] {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_collections",
			Description: "List all collections in LocalRecall",
		}, ListCollections)
		log.Println("Tool 'list_collections' enabled")
	}

	if enabledTools["list_files"] {
		if defaultCollectionName != "" {
			desc := fmt.Sprintf("List files in LocalRecall collection '%s'", defaultCollectionName)
			mcp.AddTool(server, &mcp.Tool{
				Name:        "list_files",
				Description: desc,
			}, ListFilesWithoutCollection)
			log.Printf("Tool 'list_files' enabled (using default collection: %s)", defaultCollectionName)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "list_files",
				Description: "List files in a LocalRecall collection",
			}, ListFiles)
			log.Println("Tool 'list_files' enabled")
		}
	}

	if enabledTools["delete_entry"] {
		if defaultCollectionName != "" {
			desc := fmt.Sprintf("Delete an entry from LocalRecall collection '%s'", defaultCollectionName)
			mcp.AddTool(server, &mcp.Tool{
				Name:        "delete_entry",
				Description: desc,
			}, DeleteEntryWithoutCollection)
			log.Printf("Tool 'delete_entry' enabled (using default collection: %s)", defaultCollectionName)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "delete_entry",
				Description: "Delete an entry from a LocalRecall collection",
			}, DeleteEntry)
			log.Println("Tool 'delete_entry' enabled")
		}
	}

	log.Printf("LocalRecall MCP server initialized. URL: %s", localRecallURL)
	if len(enabledTools) == 0 {
		log.Println("Warning: No tools enabled!")
	} else {
		log.Printf("Enabled %d tool(s)", len(enabledTools))
	}

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
