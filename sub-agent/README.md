# sub-agent MCP Server

A Model Context Protocol (MCP) server that allows sending chat completion messages to any OpenAI-compatible endpoint, with support for background job tracking using goroutines and in-memory storage with TTL.

## Features

- Send chat completion requests to OpenAI-compatible endpoints
- Background job tracking with asynchronous execution
- In-memory storage with configurable TTL for results
- Three MCP tools for managing sub-agent calls

## Configuration

The server is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_BASE_URL` | The base URL for the OpenAI API endpoint | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | The model to use for chat completions | `gpt-3.5-turbo` |
| `OPENAI_API_KEY` | The API key for authentication | Required |
| `TTL` | Time-to-live for stored results (Go duration format) | `1h` |

## MCP Tools

### `sub_agent_send`

Send a chat completion message to an OpenAI-compatible endpoint.

**Input:**
- `message` (string, required): The message to send to the OpenAI endpoint
- `background` (boolean, optional): Whether to run the request in the background (default: false)
- `model` (string, optional): Override the default model for this request

**Output:**
- If synchronous: Returns the completion result directly
- If background: Returns a task ID for tracking

### `sub_agent_list`

List all active sub-agent calls with their status and creation time.

**Input:** None

**Output:**
- `results`: Array of sub-agent result objects
- `count`: Number of active results

### `sub_agent_get_result`

Get the result of a completed sub-agent call by task ID.

**Input:**
- `task_id` (string, required): The task ID to retrieve the result for

**Output:**
- `result`: The sub-agent result object, or error if not found/expired

## Usage

### Building

```bash
cd sub-agent
go build -o sub-agent main.go
```

### Running

```bash
export OPENAI_BASE_URL="https://your-openai-compatible-endpoint/v1"
export OPENAI_MODEL="your-model"
export OPENAI_API_KEY="your-api-key"
export TTL="2h"

./sub-agent
```

### Example: Synchronous Request

```json
{
  "tool": "sub_agent_send",
  "arguments": {
    "message": "Hello, how are you?"
  }
}
```

### Example: Background Request

```json
{
  "tool": "sub_agent_send",
  "arguments": {
    "message": "Generate a summary of the following text...",
    "background": true
  }
}
// Returns: {"task_id": "uuid", "status": "queued"}
```

### Example: List All Jobs

```json
{
  "tool": "sub_agent_list",
  "arguments": {}
}
```

### Example: Get Result

```json
{
  "tool": "sub_agent_get_result",
  "arguments": {
    "task_id": "uuid-from-background-request"
  }
}
```

## Architecture

- **In-memory Store**: Uses a thread-safe map with read/write locks for concurrent access
- **TTL Cleanup**: Background goroutine periodically removes expired entries (runs every minute)
- **OpenAI Client**: Uses the `github.com/sashabaranov/go-openai` library for API calls
- **UUID Generation**: Uses `github.com/google/uuid` for unique task IDs

## Dependencies

- `github.com/modelcontextprotocol/go-sdk`: MCP protocol implementation
- `github.com/sashabaranov/go-openai`: OpenAI API client
- `github.com/google/uuid`: UUID generation
