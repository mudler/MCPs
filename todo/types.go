package main

// TODOItem represents a single TODO item
type TODOItem struct {
	ID        string   `json:"id"`                   // Unique identifier
	Title     string   `json:"title"`                // Task title
	Status    string   `json:"status"`               // "pending", "in_progress", "done"
	Assignee  string   `json:"assignee"`             // Agent name assigned to task
	DependsOn []string `json:"depends_on,omitempty"` // Array of TODO IDs this item depends on
}

// TODOList represents the entire TODO list
type TODOList struct {
	Items []TODOItem `json:"items"`
}

// Input types for different operations
type AddTODOInput struct {
	ID        string   `json:"id" jsonschema:"the unique ID for the TODO item (required)"`
	Title     string   `json:"title" jsonschema:"the title of the TODO item"`
	Assignee  string   `json:"assignee,omitempty" jsonschema:"the agent name assigned to this TODO item (optional)"`
	DependsOn []string `json:"depends_on,omitempty" jsonschema:"array of TODO IDs this item depends on (optional)"`
}

type UpdateTODOStatusInput struct {
	ID        string `json:"id" jsonschema:"the ID of the TODO item to update"`
	Status    string `json:"status" jsonschema:"the new status (pending, in_progress, or done)"`
	AgentName string `json:"agent_name,omitempty" jsonschema:"the name of the agent performing the update (required when not in admin mode)"`
}

type UpdateTODOAssigneeInput struct {
	ID       string `json:"id" jsonschema:"the ID of the TODO item to update"`
	Assignee string `json:"assignee" jsonschema:"the new assignee agent name"`
}

type RemoveTODOInput struct {
	ID string `json:"id" jsonschema:"the ID of the TODO item to remove"`
}

type GetTODOStatusInput struct{}

// Dependency management input types
type AddTODODependencyInput struct {
	ID        string `json:"id" jsonschema:"the ID of the TODO item"`
	DependsOn string `json:"depends_on" jsonschema:"the ID of the TODO this item depends on"`
}

type RemoveTODODependencyInput struct {
	ID        string `json:"id" jsonschema:"the ID of the TODO item"`
	DependsOn string `json:"depends_on" jsonschema:"the ID of the dependency to remove"`
}

type GetReadyTODOsInput struct{}

type GetBlockedTODOsInput struct{}

type GetTODODependenciesInput struct {
	ID         string `json:"id" jsonschema:"the ID of the TODO item"`
	Transitive bool   `json:"transitive,omitempty" jsonschema:"whether to include transitive dependencies (default: false)"`
}

// Output types
type AddTODOOutput struct {
	ID        string   `json:"id" jsonschema:"the ID of the created TODO item"`
	Title     string   `json:"title" jsonschema:"the title of the TODO item"`
	Status    string   `json:"status" jsonschema:"the status of the TODO item"`
	Assignee  string   `json:"assignee" jsonschema:"the assignee of the TODO item"`
	DependsOn []string `json:"depends_on,omitempty" jsonschema:"dependencies of the TODO item"`
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
	Blocked    int            `json:"blocked" jsonschema:"number of blocked items (pending with unsatisfied dependencies)"`
	Ready      int            `json:"ready" jsonschema:"number of ready items (pending with all dependencies satisfied)"`
	ByAssignee map[string]int `json:"by_assignee" jsonschema:"count of items by assignee"`
}

// Dependency management output types
type AddTODODependencyOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type RemoveTODODependencyOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

// BlockingInfo represents information about a blocking dependency
type BlockingInfo struct {
	ID     string `json:"id" jsonschema:"the ID of the blocking TODO"`
	Title  string `json:"title" jsonschema:"the title of the blocking TODO"`
	Status string `json:"status" jsonschema:"the status of the blocking TODO"`
}

// BlockedTODO represents a TODO that is blocked by dependencies
type BlockedTODO struct {
	ID        string         `json:"id" jsonschema:"the ID of the blocked TODO"`
	Title     string         `json:"title" jsonschema:"the title of the blocked TODO"`
	Status    string         `json:"status" jsonschema:"the status of the blocked TODO"`
	Assignee  string         `json:"assignee" jsonschema:"the assignee of the blocked TODO"`
	BlockedBy []BlockingInfo `json:"blocked_by" jsonschema:"list of blocking dependencies"`
}

type GetReadyTODOsOutput struct {
	Items []TODOItem `json:"items" jsonschema:"list of ready TODO items"`
	Count int        `json:"count" jsonschema:"number of ready items"`
}

type GetBlockedTODOsOutput struct {
	Items []BlockedTODO `json:"items" jsonschema:"list of blocked TODO items"`
	Count int           `json:"count" jsonschema:"number of blocked items"`
}

// DependencyInfo represents information about a dependency
type DependencyInfo struct {
	ID     string `json:"id" jsonschema:"the ID of the dependency"`
	Title  string `json:"title" jsonschema:"the title of the dependency"`
	Status string `json:"status" jsonschema:"the status of the dependency"`
}

type GetTODODependenciesOutput struct {
	Direct      []DependencyInfo `json:"direct" jsonschema:"direct dependencies"`
	Transitive  []DependencyInfo `json:"transitive,omitempty" jsonschema:"transitive dependencies (if requested)"`
	DirectCount int              `json:"direct_count" jsonschema:"number of direct dependencies"`
}
