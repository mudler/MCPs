package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var globalService *Service

// setGlobalService sets the global service instance (used by main.go)
func setGlobalService(service *Service) {
	globalService = service
}

// getService returns the global service instance
func getService() *Service {
	return globalService
}

// NewAddTODOHandler returns a handler configured for admin mode
func NewAddTODOHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, AddTODOInput) (*mcp.CallToolResult, AddTODOOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AddTODOInput) (*mcp.CallToolResult, AddTODOOutput, error) {
		if !adminMode {
			return nil, AddTODOOutput{}, fmt.Errorf("this action requires admin mode (set TODO_ADMIN_MODE=true): add_todo")
		}

		service := getService()
		if service == nil {
			return nil, AddTODOOutput{}, fmt.Errorf("service not initialized")
		}

		if input.ID == "" {
			return nil, AddTODOOutput{}, fmt.Errorf("TODO ID is required")
		}

		item, err := service.AddTODO(input.ID, input.Title, input.Assignee, input.DependsOn)
		if err != nil {
			return nil, AddTODOOutput{}, err
		}

		return nil, AddTODOOutput{
			ID:        item.ID,
			Title:     item.Title,
			Status:    item.Status,
			Assignee:  item.Assignee,
			DependsOn: item.DependsOn,
		}, nil
	}
}

// NewUpdateTODOStatusHandler returns handler with appropriate behavior based on admin mode
func NewUpdateTODOStatusHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, UpdateTODOStatusInput) (*mcp.CallToolResult, UpdateTODOStatusOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateTODOStatusInput) (*mcp.CallToolResult, UpdateTODOStatusOutput, error) {
		service := getService()
		if service == nil {
			return nil, UpdateTODOStatusOutput{}, fmt.Errorf("service not initialized")
		}

		var err error
		if adminMode {
			// Admin mode: allow updating any TODO
			err = service.UpdateStatus(input.ID, input.Status)
		} else {
			// Agent mode: only allow updating assigned TODOs
			if input.AgentName == "" {
				return nil, UpdateTODOStatusOutput{
					Success: false,
					Message: "agent_name is required when not in admin mode",
				}, nil
			}
			err = service.UpdateStatusWithAgent(input.ID, input.Status, input.AgentName)
		}

		if err != nil {
			return nil, UpdateTODOStatusOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}

		return nil, UpdateTODOStatusOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' status updated to '%s'", input.ID, input.Status),
		}, nil
	}
}

// NewUpdateTODOAssigneeHandler returns a handler configured for admin mode
func NewUpdateTODOAssigneeHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, UpdateTODOAssigneeInput) (*mcp.CallToolResult, UpdateTODOAssigneeOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateTODOAssigneeInput) (*mcp.CallToolResult, UpdateTODOAssigneeOutput, error) {
		if !adminMode {
			return nil, UpdateTODOAssigneeOutput{}, fmt.Errorf("this action requires admin mode (set TODO_ADMIN_MODE=true): update_todo_assignee")
		}

		service := getService()
		if service == nil {
			return nil, UpdateTODOAssigneeOutput{}, fmt.Errorf("service not initialized")
		}

		err := service.UpdateAssignee(input.ID, input.Assignee)
		if err != nil {
			return nil, UpdateTODOAssigneeOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}

		return nil, UpdateTODOAssigneeOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' assignee updated to '%s'", input.ID, input.Assignee),
		}, nil
	}
}

// NewRemoveTODOHandler returns a handler configured for admin mode
func NewRemoveTODOHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, RemoveTODOInput) (*mcp.CallToolResult, RemoveTODOOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RemoveTODOInput) (*mcp.CallToolResult, RemoveTODOOutput, error) {
		if !adminMode {
			return nil, RemoveTODOOutput{}, fmt.Errorf("this action requires admin mode (set TODO_ADMIN_MODE=true): remove_todo")
		}

		service := getService()
		if service == nil {
			return nil, RemoveTODOOutput{}, fmt.Errorf("service not initialized")
		}

		err := service.RemoveTODO(input.ID)
		if err != nil {
			return nil, RemoveTODOOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}

		return nil, RemoveTODOOutput{
			Success: true,
			Message: fmt.Sprintf("TODO item '%s' removed successfully", input.ID),
		}, nil
	}
}

// ListTODOs lists all TODO items
func ListTODOs(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (
	*mcp.CallToolResult,
	ListTODOsOutput,
	error,
) {
	service := getService()
	if service == nil {
		return nil, ListTODOsOutput{}, fmt.Errorf("service not initialized")
	}

	items, err := service.ListTODOs()
	if err != nil {
		return nil, ListTODOsOutput{}, err
	}

	return nil, ListTODOsOutput{
		Items: items,
		Count: len(items),
	}, nil
}

// GetTODOStatus returns a summary of the TODO list status
func GetTODOStatus(ctx context.Context, req *mcp.CallToolRequest, input GetTODOStatusInput) (
	*mcp.CallToolResult,
	GetTODOStatusOutput,
	error,
) {
	service := getService()
	if service == nil {
		return nil, GetTODOStatusOutput{}, fmt.Errorf("service not initialized")
	}

	summary, err := service.GetStatus()
	if err != nil {
		return nil, GetTODOStatusOutput{}, err
	}

	// Calculate blocked and ready counts efficiently
	readyItems, _ := service.GetReadyTODOs()
	blockedItems, _ := service.GetBlockedTODOs()
	ready := len(readyItems)
	blocked := len(blockedItems)

	return nil, GetTODOStatusOutput{
		Total:      summary.Total,
		Pending:    summary.Pending,
		InProgress: summary.InProgress,
		Done:       summary.Done,
		Blocked:    blocked,
		Ready:      ready,
		ByAssignee: summary.ByAssignee,
	}, nil
}

// NewAddTODODependencyHandler returns a handler configured for admin mode
func NewAddTODODependencyHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, AddTODODependencyInput) (*mcp.CallToolResult, AddTODODependencyOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input AddTODODependencyInput) (*mcp.CallToolResult, AddTODODependencyOutput, error) {
		if !adminMode {
			return nil, AddTODODependencyOutput{}, fmt.Errorf("this action requires admin mode (set TODO_ADMIN_MODE=true): add_todo_dependency")
		}

		service := getService()
		if service == nil {
			return nil, AddTODODependencyOutput{}, fmt.Errorf("service not initialized")
		}

		err := service.AddDependency(input.ID, input.DependsOn)
		if err != nil {
			return nil, AddTODODependencyOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}

		return nil, AddTODODependencyOutput{
			Success: true,
			Message: fmt.Sprintf("Dependency '%s' added to TODO '%s'", input.DependsOn, input.ID),
		}, nil
	}
}

// NewRemoveTODODependencyHandler returns a handler configured for admin mode
func NewRemoveTODODependencyHandler(adminMode bool) func(context.Context, *mcp.CallToolRequest, RemoveTODODependencyInput) (*mcp.CallToolResult, RemoveTODODependencyOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RemoveTODODependencyInput) (*mcp.CallToolResult, RemoveTODODependencyOutput, error) {
		if !adminMode {
			return nil, RemoveTODODependencyOutput{}, fmt.Errorf("this action requires admin mode (set TODO_ADMIN_MODE=true): remove_todo_dependency")
		}

		service := getService()
		if service == nil {
			return nil, RemoveTODODependencyOutput{}, fmt.Errorf("service not initialized")
		}

		err := service.RemoveDependency(input.ID, input.DependsOn)
		if err != nil {
			return nil, RemoveTODODependencyOutput{
				Success: false,
				Message: err.Error(),
			}, nil
		}

		return nil, RemoveTODODependencyOutput{
			Success: true,
			Message: fmt.Sprintf("Dependency '%s' removed from TODO '%s'", input.DependsOn, input.ID),
		}, nil
	}
}

// GetReadyTODOs returns TODOs that are ready to start
func GetReadyTODOs(ctx context.Context, req *mcp.CallToolRequest, input GetReadyTODOsInput) (
	*mcp.CallToolResult,
	GetReadyTODOsOutput,
	error,
) {
	service := getService()
	if service == nil {
		return nil, GetReadyTODOsOutput{}, fmt.Errorf("service not initialized")
	}

	items, err := service.GetReadyTODOs()
	if err != nil {
		return nil, GetReadyTODOsOutput{}, err
	}

	return nil, GetReadyTODOsOutput{
		Items: items,
		Count: len(items),
	}, nil
}

// GetBlockedTODOs returns TODOs that are blocked by dependencies
func GetBlockedTODOs(ctx context.Context, req *mcp.CallToolRequest, input GetBlockedTODOsInput) (
	*mcp.CallToolResult,
	GetBlockedTODOsOutput,
	error,
) {
	service := getService()
	if service == nil {
		return nil, GetBlockedTODOsOutput{}, fmt.Errorf("service not initialized")
	}

	items, err := service.GetBlockedTODOs()
	if err != nil {
		return nil, GetBlockedTODOsOutput{}, err
	}

	return nil, GetBlockedTODOsOutput{
		Items: items,
		Count: len(items),
	}, nil
}

// GetTODODependencies returns dependencies for a TODO
func GetTODODependencies(ctx context.Context, req *mcp.CallToolRequest, input GetTODODependenciesInput) (
	*mcp.CallToolResult,
	GetTODODependenciesOutput,
	error,
) {
	service := getService()
	if service == nil {
		return nil, GetTODODependenciesOutput{}, fmt.Errorf("service not initialized")
	}

	result, err := service.GetDependencies(input.ID, input.Transitive)
	if err != nil {
		return nil, GetTODODependenciesOutput{}, err
	}

	return nil, *result, nil
}
