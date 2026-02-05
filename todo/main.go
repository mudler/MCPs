package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TODOItem represents a single TODO item
type TODOItem struct {
	ID       string `json:"id"`       // Unique identifier
	Title    string `json:"title"`    // Task title
	Status   string `json:"status"`   // "pending", "in_progress", "done"
	Assignee string `json:"assignee"` // Agent name assigned to task
}

// TODOList represents the entire TODO list
type TODOList struct {
	Items []TODOItem `json:"items"`
}

// Input types for different operations
type AddTODOInput struct {
	Title    string `json:"title" jsonschema:"the title of the TODO item"`
	Assignee string `json:"assignee,omitempty" jsonschema:"the agent name assigned to this TODO item (optional)"`
}

type UpdateTODOStatusInput struct {
	ID     string `json:"id" jsonschema:"the ID of the TODO item to update"`
	Status string `json:"status" jsonschema:"the new status (pending, in_progress, or done)"`
}

type UpdateTODOAssigneeInput struct {
	ID       string `json:"id" jsonschema:"the ID of the TODO item to update"`
	Assignee string `json:"assignee" jsonschema:"the new assignee agent name"`
}

type RemoveTODOInput struct {
	ID string `json:"id" jsonschema:"the ID of the TODO item to remove"`
}

type GetTODOStatusInput struct{}

// Output types
type AddTODOOutput struct {
	ID       string `json:"id" jsonschema:"the ID of the created TODO item"`
	Title    string `json:"title" jsonschema:"the title of the TODO item"`
	Status   string `json:"status" jsonschema:"the status of the TODO item"`
	Assignee string `json:"assignee" jsonschema:"the assignee of the TODO item"`
}

type ListTODOsOutput struct {
	Items []TODOItem `json:"items" jsonschema:"list of all TODO items"`
	Count int        `json:"count" jsonschema:"number of TODO items"`
}

type UpdateTODOStatusOutput struct {
	Success bool   `json:"success" jsonschema:"whether the update was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type UpdateTODOAssigneeOutput struct {
	Success bool   `json:"success" jsonschema:"whether the update was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type RemoveTODOOutput struct {
	Success bool   `json:"success" jsonschema:"whether the removal was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type GetTODOStatusOutput struct {
	Total      int            `json:"total" jsonschema:"total number of TODO items"`
	Pending    int            `json:"pending" jsonschema:"number of pending items"`
	InProgress int            `json:"in_progress" jsonschema:"number of in_progress items"`
	Done       int            `json:"done" jsonschema:"number of done items"`
	ByAssignee map[string]int `json:"by_assignee" jsonschema:"count of items by assignee"`
}

var todoFilePath string

// Generate a unique ID for TODO items
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// withLock executes a function with file locking
func withLock(filePath string, fn func() error) error {
	lockPath := filePath + ".lock"
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

// loadTODOs loads the TODO list from file
func loadTODOs() (*TODOList, error) {
	list := &TODOList{Items: []TODOItem{}}

	// Check if file exists
	if _, err := os.Stat(todoFilePath); os.IsNotExist(err) {
		// File doesn't exist, return empty list
		return list, nil
	}

	data, err := os.ReadFile(todoFilePath)
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

// saveTODOs saves the TODO list to file atomically
func saveTODOs(list *TODOList) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(todoFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first
	tempFile := todoFilePath + ".tmp"
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal TODO list: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, todoFilePath); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// AddTODO adds a new TODO item
func AddTODO(ctx context.Context, req *mcp.CallToolRequest, input AddTODOInput) (
	*mcp.CallToolResult,
	AddTODOOutput,
	error,
) {
	if input.Title == "" {
		return nil, AddTODOOutput{}, fmt.Errorf("title is required")
	}

	var output AddTODOOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		item := TODOItem{
			ID:       generateID(),
			Title:    input.Title,
			Status:   "pending",
			Assignee: input.Assignee,
		}

		list.Items = append(list.Items, item)

		if err := saveTODOs(list); err != nil {
			return err
		}

		output = AddTODOOutput{
			ID:       item.ID,
			Title:    item.Title,
			Status:   item.Status,
			Assignee: item.Assignee,
		}

		return nil
	})

	if err != nil {
		return nil, AddTODOOutput{}, err
	}

	return nil, output, nil
}

// UpdateTODOStatus updates the status of a TODO item
func UpdateTODOStatus(ctx context.Context, req *mcp.CallToolRequest, input UpdateTODOStatusInput) (
	*mcp.CallToolResult,
	UpdateTODOStatusOutput,
	error,
) {
	// Validate status
	validStatuses := map[string]bool{"pending": true, "in_progress": true, "done": true}
	if !validStatuses[input.Status] {
		return nil, UpdateTODOStatusOutput{}, fmt.Errorf("invalid status: %s (must be pending, in_progress, or done)", input.Status)
	}

	var output UpdateTODOStatusOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		found := false
		for i := range list.Items {
			if list.Items[i].ID == input.ID {
				list.Items[i].Status = input.Status
				found = true
				break
			}
		}

		if !found {
			output = UpdateTODOStatusOutput{
				Success: false,
				Message: fmt.Sprintf("TODO item with ID '%s' not found", input.ID),
			}
			return nil
		}

		if err := saveTODOs(list); err != nil {
			return err
		}

		output = UpdateTODOStatusOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' status updated to '%s'", input.ID, input.Status),
		}

		return nil
	})

	if err != nil {
		return nil, UpdateTODOStatusOutput{}, err
	}

	return nil, output, nil
}

// UpdateTODOAssignee updates the assignee of a TODO item
func UpdateTODOAssignee(ctx context.Context, req *mcp.CallToolRequest, input UpdateTODOAssigneeInput) (
	*mcp.CallToolResult,
	UpdateTODOAssigneeOutput,
	error,
) {
	var output UpdateTODOAssigneeOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		found := false
		for i := range list.Items {
			if list.Items[i].ID == input.ID {
				list.Items[i].Assignee = input.Assignee
				found = true
				break
			}
		}

		if !found {
			output = UpdateTODOAssigneeOutput{
				Success: false,
				Message: fmt.Sprintf("TODO item with ID '%s' not found", input.ID),
			}
			return nil
		}

		if err := saveTODOs(list); err != nil {
			return err
		}

		output = UpdateTODOAssigneeOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' assignee updated to '%s'", input.ID, input.Assignee),
		}

		return nil
	})

	if err != nil {
		return nil, UpdateTODOAssigneeOutput{}, err
	}

	return nil, output, nil
}

// RemoveTODO removes a TODO item by ID
func RemoveTODO(ctx context.Context, req *mcp.CallToolRequest, input RemoveTODOInput) (
	*mcp.CallToolResult,
	RemoveTODOOutput,
	error,
) {
	var output RemoveTODOOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		found := false
		newItems := []TODOItem{}
		for _, item := range list.Items {
			if item.ID == input.ID {
				found = true
			} else {
				newItems = append(newItems, item)
			}
		}

		if !found {
			output = RemoveTODOOutput{
				Success: false,
				Message: fmt.Sprintf("TODO item with ID '%s' not found", input.ID),
			}
			return nil
		}

		list.Items = newItems

		if err := saveTODOs(list); err != nil {
			return err
		}

		output = RemoveTODOOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' removed successfully", input.ID),
		}

		return nil
	})

	if err != nil {
		return nil, RemoveTODOOutput{}, err
	}

	return nil, output, nil
}

// ListTODOs lists all TODO items
func ListTODOs(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (
	*mcp.CallToolResult,
	ListTODOsOutput,
	error,
) {
	var output ListTODOsOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		output = ListTODOsOutput{
			Items: list.Items,
			Count: len(list.Items),
		}

		return nil
	})

	if err != nil {
		return nil, ListTODOsOutput{}, err
	}

	return nil, output, nil
}

// GetTODOStatus returns a summary of the TODO list status
func GetTODOStatus(ctx context.Context, req *mcp.CallToolRequest, input GetTODOStatusInput) (
	*mcp.CallToolResult,
	GetTODOStatusOutput,
	error,
) {
	var output GetTODOStatusOutput

	err := withLock(todoFilePath, func() error {
		list, err := loadTODOs()
		if err != nil {
			return err
		}

		status := GetTODOStatusOutput{
			Total:      len(list.Items),
			Pending:    0,
			InProgress: 0,
			Done:       0,
			ByAssignee: make(map[string]int),
		}

		for _, item := range list.Items {
			switch item.Status {
			case "pending":
				status.Pending++
			case "in_progress":
				status.InProgress++
			case "done":
				status.Done++
			}

			if item.Assignee != "" {
				status.ByAssignee[item.Assignee]++
			}
		}

		output = status
		return nil
	})

	if err != nil {
		return nil, GetTODOStatusOutput{}, err
	}

	return nil, output, nil
}

func main() {
	// Get file path from environment variable, default to /data/todos.json
	todoFilePath = os.Getenv("TODO_FILE_PATH")
	if todoFilePath == "" {
		todoFilePath = "/data/todos.json"
	}

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(todoFilePath), 0755)

	// Create a server with TODO tools
	server := mcp.NewServer(&mcp.Implementation{Name: "todo", Version: "v1.0.0"}, nil)

	// Register TODO tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_todo",
		Description: "Add a new TODO item to the shared TODO list",
	}, AddTODO)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_todo_status",
		Description: "Update the status of a TODO item (pending, in_progress, or done)",
	}, UpdateTODOStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_todo_assignee",
		Description: "Update the assignee of a TODO item",
	}, UpdateTODOAssignee)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_todo",
		Description: "Remove a TODO item by ID",
	}, RemoveTODO)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_todos",
		Description: "List all TODO items",
	}, ListTODOs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_todo_status",
		Description: "Get a summary of the TODO list status with counts by status and assignee",
	}, GetTODOStatus)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
