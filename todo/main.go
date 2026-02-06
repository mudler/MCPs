package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var todoFilePath string

func main() {
	// Get file path from environment variable, default to /data/todos.json
	todoFilePath = os.Getenv("TODO_FILE_PATH")
	if todoFilePath == "" {
		todoFilePath = "/data/todos.json"
	}

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(todoFilePath), 0755)

	// Create storage and service
	storage := NewFileStorage(todoFilePath)
	service := NewService(storage)
	setGlobalService(service)

	// Check admin mode once at startup
	adminMode := isAdminMode()

	// Create a server with TODO tools
	server := mcp.NewServer(&mcp.Implementation{Name: "todo", Version: "v1.0.0"}, nil)

	// Register admin-only tools conditionally
	if adminMode {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "add_todo",
			Description: "Add a new TODO item to the shared TODO list",
		}, NewAddTODOHandler(adminMode))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "update_todo_assignee",
			Description: "Update the assignee of a TODO item",
		}, NewUpdateTODOAssigneeHandler(adminMode))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "remove_todo",
			Description: "Remove a TODO item by ID",
		}, NewRemoveTODOHandler(adminMode))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "add_todo_dependency",
			Description: "Add a dependency to a TODO item",
		}, NewAddTODODependencyHandler(adminMode))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "remove_todo_dependency",
			Description: "Remove a dependency from a TODO item",
		}, NewRemoveTODODependencyHandler(adminMode))
	}

	// Register always-available tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_todo_status",
		Description: "Update the status of a TODO item (pending, in_progress, or done)",
	}, NewUpdateTODOStatusHandler(adminMode))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_todos",
		Description: "List all TODO items",
	}, ListTODOs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_todo_status",
		Description: "Get a summary of the TODO list status with counts by status and assignee",
	}, GetTODOStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ready_todos",
		Description: "Get all TODO items that are ready to start (pending with all dependencies satisfied)",
	}, GetReadyTODOs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_blocked_todos",
		Description: "Get all TODO items that are blocked by dependencies",
	}, GetBlockedTODOs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_todo_dependencies",
		Description: "Get dependencies for a TODO item (direct and optionally transitive)",
	}, GetTODODependencies)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
