package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SubAgentJob represents a background job tracked by the sub-agent MCP server
type SubAgentJob struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"` // pending, running, completed, failed
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	TTL       time.Duration     `json:"ttl,omitempty"`
	Inputs    map[string]string `json:"inputs,omitempty"`
	Result    string            `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// SubAgentManager handles job tracking and lifecycle
type SubAgentManager struct {
	jobs   map[string]*SubAgentJob
	mu     sync.RWMutex
	ttl    time.Duration
	baseURL string
	model   string
	apiKey  string
}

// NewSubAgentManager creates a new manager with configurable TTL
func NewSubAgentManager() *SubAgentManager {
	ttl := 24 * time.Hour // default TTL
	if ttlStr := os.Getenv("SUB_AGENT_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			ttl = d
		}
	}

	return &SubAgentManager{
		jobs:    make(map[string]*SubAgentJob),
		ttl:     ttl,
		baseURL: os.Getenv("OPENAI_BASE_URL"),
		model:   os.Getenv("OPENAI_MODEL"),
		apiKey:  os.Getenv("OPENAI_API_KEY"),
	}
}

// CreateJob creates a new sub-agent job
func (m *SubAgentManager) CreateJob(id string, inputs map[string]string) *SubAgentJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &SubAgentJob{
		ID:        id,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       m.ttl,
		Inputs:    inputs,
	}
	m.jobs[id] = job
	return job
}

// GetJob retrieves a job by ID
func (m *SubAgentManager) GetJob(id string) (*SubAgentJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	return job, exists
}

// ListJobs returns all jobs with optional status filter
func (m *SubAgentManager) ListJobs(status string) []*SubAgentJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SubAgentJob
	now := time.Now()
	for _, job := range m.jobs {
		// Check TTL
		if now.Sub(job.UpdatedAt) > job.TTL {
			continue
		}
		if status == "" || job.Status == status {
			result = append(result, job)
		}
	}
	return result
}

// UpdateJob updates a job's status and result
func (m *SubAgentManager) UpdateJob(id, status, result, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[id]
	if !exists {
		return fmt.Errorf("job %s not found", id)
	}

	job.Status = status
	job.UpdatedAt = time.Now()
	job.Result = result
	if errMsg != "" {
		job.Error = errMsg
	}
	m.jobs[id] = job
	return nil
}

// StartJob marks a job as running
func (m *SubAgentManager) StartJob(id string) error {
	return m.UpdateJob(id, "running", "", "")
}

// CompleteJob marks a job as completed
func (m *SubAgentManager) CompleteJob(id, result string) error {
	return m.UpdateJob(id, "completed", result, "")
}

// FailJob marks a job as failed
func (m *SubAgentManager) FailJob(id, errMsg string) error {
	return m.UpdateJob(id, "failed", "", errMsg)
}

// CallOpenAIChatCompletion sends a chat completion request to OpenAI API
func (m *SubAgentManager) CallOpenAIChatCompletion(ctx context.Context, req *mcp.CallToolRequest, input struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Model string `json:"model,omitempty"`
}) (*mcp.CallToolResult, struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}, error) {
	model := m.model
	if input.Model != "" {
		model = input.Model
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	// Build OpenAI API request
	type openAIRequest struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	oaReq := openAIRequest{
		Model:    model,
		Messages: make([]struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}, len(input.Messages)),
	}

	for i, msg := range input.Messages {
		oaReq.Messages[i].Role = msg.Role
		oaReq.Messages[i].Content = msg.Content
	}

	reqBody, err := json.Marshal(oaReq)
	if err != nil {
		return nil, struct {
			Content string `json:"content"`
			Error   string `json:"error,omitempty"`
		}{Error: fmt.Sprintf("Failed to marshal request: %v", err)}, nil
	}

	// Execute curl command to call OpenAI API
	baseURL := m.baseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1/chat/completions"
	}

	curlCmd := exec.CommandContext(ctx, "curl", "-s",
		"-X", "POST", baseURL,
		"-H", "Content-Type: application/json",
		"-H", fmt.Sprintf("Authorization: Bearer %s", m.apiKey),
		"-d", string(reqBody),
	)

	output, err := curlCmd.Output()
	if err != nil {
		return nil, struct {
			Content string `json:"content"`
			Error   string `json:"error,omitempty"`
		}{Error: fmt.Sprintf("API call failed: %v", err)}, nil
	}

	// Parse response
	type openAIResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	var oaResp openAIResponse
	if err := json.Unmarshal(output, &oaResp); err != nil {
		return nil, struct {
			Content string `json:"content"`
			Error   string `json:"error,omitempty"`
		}{Error: fmt.Sprintf("Failed to parse response: %v", err)}, nil
	}

	if oaResp.Error.Message != "" {
		return nil, struct {
			Content string `json:"content"`
			Error   string `json:"error,omitempty"`
		}{Error: oaResp.Error.Message}, nil
	}

	content := ""
	if len(oaResp.Choices) > 0 {
		content = oaResp.Choices[0].Message.Content
	}

	return nil, struct {
		Content string `json:"content"`
		Error   string `json:"error,omitempty"`
	}{Content: content}, nil
}

// CreateJobTool creates a job
func CreateJobTool(ctx context.Context, req *mcp.CallToolRequest, input struct {
	ID     string            `json:"id" jsonschema:"unique job identifier"`
	Inputs map[string]string `json:"inputs,omitempty" jsonschema:"optional input parameters"`
}) (*mcp.CallToolResult, struct {
	JobID string `json:"job_id"`
	Status string `json:"status"`
}, error) {
	manager := NewSubAgentManager()
	job := manager.CreateJob(input.ID, input.Inputs)
	return nil, struct {
		JobID string `json:"job_id"`
		Status string `json:"status"`
	}{JobID: job.ID, Status: job.Status}, nil
}

// GetJobTool retrieves a job
func GetJobTool(ctx context.Context, req *mcp.CallToolRequest, input struct {
	ID string `json:"id" jsonschema:"job identifier"`
}) (*mcp.CallToolResult, SubAgentJob, error) {
	manager := NewSubAgentManager()
	job, exists := manager.GetJob(input.ID)
	if !exists {
		return nil, SubAgentJob{}, fmt.Errorf("job not found")
	}
	return nil, *job, nil
}

// ListJobsTool lists jobs
func ListJobsTool(ctx context.Context, req *mcp.CallToolRequest, input struct {
	Status string `json:"status,omitempty" jsonschema:"optional status filter"`
}) (*mcp.CallToolResult, struct {
	Jobs []*SubAgentJob `json:"jobs"`
}, error) {
	manager := NewSubAgentManager()
	jobs := manager.ListJobs(input.Status)
	return nil, struct {
		Jobs []*SubAgentJob `json:"jobs"`
	}{Jobs: jobs}, nil
}

// UpdateJobTool updates a job
func UpdateJobTool(ctx context.Context, req *mcp.CallToolRequest, input struct {
	ID     string `json:"id" jsonschema:"job identifier"`
	Status string `json:"status" jsonschema:"new status (running, completed, failed)"`
	Result string `json:"result,omitempty" jsonschema:"job result"`
	Error  string `json:"error,omitempty" jsonschema:"error message if failed"`
}) (*mcp.CallToolResult, struct {
	JobID string `json:"job_id"`
	Status string `json:"status"`
}, error) {
	manager := NewSubAgentManager()
	var err error
	switch input.Status {
	case "running":
		err = manager.StartJob(input.ID)
	case "completed":
		err = manager.CompleteJob(input.ID, input.Result)
	case "failed":
		err = manager.FailJob(input.ID, input.Error)
	default:
		err = manager.UpdateJob(input.ID, input.Status, input.Result, input.Error)
	}
	if err != nil {
		return nil, struct {
			JobID string `json:"job_id"`
			Status string `json:"status"`
		}{}, err
	}
	return nil, struct {
		JobID string `json:"job_id"`
		Status string `json:"status"`
	}{JobID: input.ID, Status: input.Status}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "sub-agent",
		Version: "v1.0.0",
	}, nil)

	// Add tool for creating jobs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_job",
		Description: "Create a new sub-agent job with optional inputs. The job will be tracked with a configurable TTL.",
	}, CreateJobTool)

	// Add tool for getting a job
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_job",
		Description: "Retrieve a sub-agent job by its ID.",
	}, GetJobTool)

	// Add tool for listing jobs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_jobs",
		Description: "List all sub-agent jobs, optionally filtered by status. Jobs older than their TTL are automatically excluded.",
	}, ListJobsTool)

	// Add tool for updating a job
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_job",
		Description: "Update a job's status and result. Status can be 'running', 'completed', or 'failed'.",
	}, UpdateJobTool)

	// Add tool for OpenAI chat completion
	mcp.AddTool(server, &mcp.Tool{
		Name:        "chat_completion",
		Description: "Send a chat completion request to OpenAI API. Uses OPENAI_BASE_URL, OPENAI_MODEL, and OPENAI_API_KEY environment variables.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Model string `json:"model,omitempty"`
	}) (*mcp.CallToolResult, struct {
		Content string `json:"content"`
		Error   string `json:"error,omitempty"`
	}, error) {
		manager := NewSubAgentManager()
		return manager.CallOpenAIChatCompletion(ctx, req, input)
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
