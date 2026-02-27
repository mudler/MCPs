package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Task represents a background processing task
type Task struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Result    string    `json:"result,omitempty"`
	Status    string    `json:"status"` // "pending", "completed", "expired"
	CreatedAt time.Time `json:"created_at"`
}

// TaskStore manages background tasks with TTL
type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	ttl   time.Duration
}

// NewTaskStore creates a new task store with specified TTL
func NewTaskStore(ttl time.Duration) *TaskStore {
	store := &TaskStore{
		tasks: make(map[string]*Task),
		ttl:   ttl,
	}
	// Start cleanup goroutine
	go store.cleanupLoop()
	return store
}

// cleanupLoop periodically removes expired tasks
func (s *TaskStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanupExpired()
	}
}

// cleanupExpired removes all expired tasks
func (s *TaskStore) cleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, task := range s.tasks {
		if now.Sub(task.CreatedAt) > s.ttl {
			task.Status = "expired"
			delete(s.tasks, id)
		}
	}
}

// AddTask adds a new task to the store
func (s *TaskStore) AddTask(id, message string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := &Task{
		ID:        id,
		Message:   message,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	s.tasks[id] = task
	return task
}

// GetTask retrieves a task by ID
func (s *TaskStore) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	// Check if expired
	if time.Since(task.CreatedAt) > s.ttl {
		return nil, fmt.Errorf("task expired: %s", id)
	}
	return task, nil
}

// ListTasks returns all active tasks
func (s *TaskStore) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Task
	for _, task := range s.tasks {
		// Skip expired tasks
		if time.Since(task.CreatedAt) <= s.ttl {
			result = append(result, task)
		}
	}
	return result
}

// SetResult sets the result for a task
func (s *TaskStore) SetResult(id, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}
	task.Result = result
	task.Status = "completed"
	return nil
}

// ChatInput represents the input for sub_agent_chat tool
type ChatInput struct {
	Message   string `json:"message" jsonschema:"the message to send to the AI model"`
	Background *bool  `json:"background,omitempty" jsonschema:"whether to process in background (defaults to SUB_AGENT_BACKGROUND_DEFAULT env var, or false if not set)"`
}

// ChatOutput represents the output of sub_agent_chat tool
type ChatOutput struct {
	Response string `json:"response,omitempty" jsonschema:"the AI response (if not background)"`
	TaskID   string `json:"task_id,omitempty" jsonschema:"the task ID for background processing"`
	Status   string `json:"status" jsonschema:"status of the operation"`
}

// TaskInfo represents task info for listing
type TaskInfo struct {
	TaskID    string `json:"task_id" jsonschema:"the task ID"`
	CreatedAt string `json:"created_at" jsonschema:"when the task was created"`
	Status    string `json:"status" jsonschema:"current status of the task"`
	Message   string `json:"message" jsonschema:"the original message"`
}

// GetResultInput represents input for sub_agent_get_result tool
type GetResultInput struct {
	TaskID string `json:"task_id" jsonschema:"the task ID to get result for"`
}

// GetResultOutput represents output for sub_agent_get_result tool
type GetResultOutput struct {
	TaskID    string `json:"task_id" jsonschema:"the task ID"`
	Result    string `json:"result" jsonschema:"the task result"`
	Status    string `json:"status" jsonschema:"task status"`
	Message   string `json:"message" jsonschema:"the original message"`
	CreatedAt string `json:"created_at" jsonschema:"when the task was created"`
}

var (
	openaiBaseURL        string
	openaiModel          string
	openaiAPIKey         string
	subAgentTTL          time.Duration
	taskStore            *TaskStore
	backgroundDefault    bool
)

func initConfig() {
	openaiBaseURL = os.Getenv("OPENAI_BASE_URL")
	if openaiBaseURL == "" {
		openaiBaseURL = "https://api.openai.com/v1"
	}
	openaiModel = os.Getenv("OPENAI_MODEL")
	if openaiModel == "" {
		openaiModel = "gpt-4o-mini"
	}
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")

	ttlHours := 4
	if ttlEnv := os.Getenv("SUB_AGENT_TTL"); ttlEnv != "" {
		fmt.Sscanf(ttlEnv, "%d", &ttlHours)
	}
	subAgentTTL = time.Duration(ttlHours) * time.Hour
	taskStore = NewTaskStore(subAgentTTL)

	// Read background default from environment variable
	// SUB_AGENT_BACKGROUND_DEFAULT can be "true" or "false"
	// If not set or invalid, defaults to false
	backgroundDefault = false
	if bgEnv := os.Getenv("SUB_AGENT_BACKGROUND_DEFAULT"); bgEnv != "" {
		backgroundDefault, _ = strconv.ParseBool(bgEnv)
	}
}

func callOpenAI(ctx context.Context, message string) (string, error) {
	// Check if API key is set
	if openaiAPIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// For now, return a simulated response
	// In a full implementation, you would make an actual HTTP request to OpenAI
	return fmt.Sprintf("Processed: %s", message), nil
}

func SubAgentChat(ctx context.Context, req *mcp.CallToolRequest, input ChatInput) (*mcp.CallToolResult, ChatOutput, error) {
	if input.Message == "" {
		return nil, ChatOutput{}, fmt.Errorf("message cannot be empty")
	}

	// Use environment variable default, but allow request-level override
	background := backgroundDefault
	if input.Background != nil {
		background = *input.Background
	}

	if background {
		// Generate task ID
		taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

		// Store the task
		taskStore.AddTask(taskID, input.Message)

		// Process in background
		go func() {
			result, err := callOpenAI(ctx, input.Message)
			if err != nil {
				log.Printf("Error processing task %s: %v", taskID, err)
				taskStore.SetResult(taskID, fmt.Sprintf("Error: %v", err))
			} else {
				taskStore.SetResult(taskID, result)
			}
		}()

		return nil, ChatOutput{
			TaskID: taskID,
			Status: "processing",
		}, nil
	}

	// Synchronous processing
	response, err := callOpenAI(ctx, input.Message)
	if err != nil {
		return nil, ChatOutput{}, err
	}

	return nil, ChatOutput{
		Response: response,
		Status:   "completed",
	}, nil
}

func SubAgentList(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, []TaskInfo, error) {
	tasks := taskStore.ListTasks()
	var result []TaskInfo
	for _, task := range tasks {
		result = append(result, TaskInfo{
			TaskID:    task.ID,
			CreatedAt: task.CreatedAt.Format(time.RFC3339),
			Status:    task.Status,
			Message:   task.Message,
		})
	}

	return nil, result, nil
}

func SubAgentGetResult(ctx context.Context, req *mcp.CallToolRequest, input GetResultInput) (*mcp.CallToolResult, GetResultOutput, error) {
	if input.TaskID == "" {
		return nil, GetResultOutput{}, fmt.Errorf("task_id cannot be empty")
	}

	task, err := taskStore.GetTask(input.TaskID)
	if err != nil {
		return nil, GetResultOutput{}, err
	}

	return nil, GetResultOutput{
		TaskID:    task.ID,
		Result:    task.Result,
		Status:    task.Status,
		Message:   task.Message,
		CreatedAt: task.CreatedAt.Format(time.RFC3339),
	}, nil
}

func main() {
	initConfig()

	server := mcp.NewServer(&mcp.Implementation{Name: "sub-agent", Version: "v1.0.0"}, nil)

	// Add sub_agent_chat tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_chat",
		Description: "Send a chat completion message to OpenAI. Can process synchronously or in background.",
	}, SubAgentChat)

	// Add sub_agent_list tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_list",
		Description: "List all active background sub-agent calls",
	}, SubAgentList)

	// Add sub_agent_get_result tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_get_result",
		Description: "Get the result of a background task",
	}, SubAgentGetResult)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
