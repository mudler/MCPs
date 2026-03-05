package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sashabaranov/go-openai"
)

// SubAgentResult represents the result of a sub-agent call
type SubAgentResult struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Request   string    `json:"request"`
	Response  string    `json:"response,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SubAgentStore is an in-memory store with TTL for tracking sub-agent calls
type SubAgentStore struct {
	mu    sync.RWMutex
	items map[string]*SubAgentResult
	ttl   time.Duration
}

// NewSubAgentStore creates a new SubAgentStore with the given TTL
func NewSubAgentStore(ttl time.Duration) *SubAgentStore {
	store := &SubAgentStore{
		items: make(map[string]*SubAgentResult),
		ttl:   ttl,
	}
	// Start cleanup goroutine
	go store.cleanupLoop()
	return store
}

// cleanupLoop periodically removes expired entries
func (s *SubAgentStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes expired entries from the store
func (s *SubAgentStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, item := range s.items {
		if now.Sub(item.CreatedAt) > s.ttl {
			delete(s.items, id)
		}
	}
}

// Add adds a new result to the store
func (s *SubAgentStore) Add(result *SubAgentResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[result.ID] = result
}

// Get retrieves a result by ID
func (s *SubAgentStore) Get(id string) (*SubAgentResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, exists := s.items[id]
	if !exists {
		return nil, false
	}
	// Check TTL
	if time.Since(item.CreatedAt) > s.ttl {
		return nil, false
	}
	return item, true
}

// List returns all active results
func (s *SubAgentStore) List() []*SubAgentResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []*SubAgentResult
	now := time.Now()
	for _, item := range s.items {
		if now.Sub(item.CreatedAt) <= s.ttl {
			results = append(results, item)
		}
	}
	return results
}

var (
	store      *SubAgentStore
	openaiClient *openai.Client
	openaiModel  string
)

func init() {
	// Initialize TTL (default 1 hour)
	ttl := 1 * time.Hour
	if ttlStr := os.Getenv("TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			ttl = d
		}
	}
	store = NewSubAgentStore(ttl)

	// Initialize OpenAI client
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	openaiClient = openai.NewClientWithConfig(config)

	// Set default model
	openaiModel = os.Getenv("OPENAI_MODEL")
	if openaiModel == "" {
		openaiModel = "gpt-3.5-turbo"
	}
}

// SendInput represents the input for sub_agent_send
type SendInput struct {
	Message    string `json:"message" jsonschema:"the message to send to the OpenAI endpoint"`
	Background bool   `json:"background,omitempty" jsonschema:"whether to run the request in the background (default: false)"`
	Model      string `json:"model,omitempty" jsonschema:"override the default model for this request"`
}

// SendOutput represents the output for sub_agent_send
type SendOutput struct {
	TaskID string `json:"task_id,omitempty"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ListInput represents the input for sub_agent_list
type ListInput struct{}

// ListOutput represents the output for sub_agent_list
type ListOutput struct {
	Results []SubAgentResult `json:"results"`
	Count   int              `json:"count"`
}

// GetInput represents the input for sub_agent_get_result
type GetInput struct {
	TaskID string `json:"task_id" jsonschema:"the task ID to retrieve the result for"`
}

// GetOutput represents the output for sub_agent_get_result
type GetOutput struct {
	Result *SubAgentResult `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// SendChatCompletion sends a chat completion request
func SendChatCompletion(ctx context.Context, req *mcp.CallToolRequest, input SendInput) (*mcp.CallToolResult, SendOutput, error) {
	model := openaiModel
	if input.Model != "" {
		model = input.Model
	}

	resp, err := openaiClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: input.Message,
			},
		},
	})

	if err != nil {
		return nil, SendOutput{
			Status: "error",
			Error:  err.Error(),
		}, nil
	}

	result := ""
	if len(resp.Choices) > 0 {
		result = resp.Choices[0].Message.Content
	}

	if input.Background {
		taskID := uuid.New().String()
		subResult := &SubAgentResult{
			ID:        taskID,
			Status:    "completed",
			Request:   input.Message,
			Response:  result,
			CreatedAt: time.Now(),
		}
		store.Add(subResult)

		return nil, SendOutput{
			TaskID: taskID,
			Status: "queued",
			Result: "Background job created",
		}, nil
	}

	return nil, SendOutput{
		Status: "completed",
		Result: result,
	}, nil
}

// ListSubAgentCalls lists all active sub-agent calls
func ListSubAgentCalls(ctx context.Context, req *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, ListOutput, error) {
	results := store.List()
	return nil, ListOutput{
		Results: func() []SubAgentResult {
			res := make([]SubAgentResult, len(results))
			for i, r := range results {
				res[i] = *r
			}
			return res
		}(),
		Count: len(results),
	}, nil
}

// GetSubAgentResult retrieves a specific sub-agent result
func GetSubAgentResult(ctx context.Context, req *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, GetOutput, error) {
	result, exists := store.Get(input.TaskID)
	if !exists {
		return nil, GetOutput{
			Error: "Task not found or has expired",
		}, nil
	}
	return nil, GetOutput{
		Result: result,
	}, nil
}

func main() {
	// Create a server with the sub-agent tools
	server := mcp.NewServer(&mcp.Implementation{Name: "sub-agent", Version: "v1.0.0"}, nil)

	// Add sub_agent_send tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_send",
		Description: "Send a chat completion message to an OpenAI-compatible endpoint. Can run in background mode to track results asynchronously.",
	}, SendChatCompletion)

	// Add sub_agent_list tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_list",
		Description: "List all active sub-agent calls with their status and creation time.",
	}, ListSubAgentCalls)

	// Add sub_agent_get_result tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sub_agent_get_result",
		Description: "Get the result of a completed sub-agent call by task ID.",
	}, GetSubAgentResult)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
