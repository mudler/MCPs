# Sub-Agent MCP Server

A Model Context Protocol (MCP) server that provides chat completion capabilities with background processing support.

## Features

- **Chat Completion Tool**: Send messages to OpenAI API with synchronous or asynchronous processing
- **Background Processing**: Execute chat completions in the background and retrieve results later
- **Task Management**: List active background tasks and get their results
- **TTL-based Cleanup**: Automatic cleanup of expired tasks based on configurable TTL

## Installation

### Building

```bash
cd sub-agent
go build -o sub-agent .
```

### Running

```bash
./sub-agent
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_BASE_URL` | Base URL for OpenAI API | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Model to use for completions | `gpt-4o-mini` |
| `OPENAI_API_KEY` | API key for OpenAI authentication | (required) |
| `SUB_AGENT_TTL` | TTL in hours for background task results | `4` |

## Tools

### `sub_agent_chat`

Send a chat completion message to OpenAI.

**Inputs:**
- `message` (string, required): The message to send to the AI model
- `background` (boolean, optional): Whether to process in background (default: false)

**Outputs:**
- If `background` is false: Returns the AI response immediately
- If `background` is true: Returns a task ID for later retrieval

**Example (synchronous):**
```json
{
  "message": "What is the capital of France?"
}
```

**Example (background):**
```json
{
  "message": "Analyze this long document...",
  "background": true
}
```

### `sub_agent_list`

List all active background sub-agent calls.

**Inputs:** None

**Outputs:** List of task objects with task_id, created_at, status, and message

**Example:**
```json
[
  {
    "task_id": "task-1234567890",
    "created_at": "2024-01-01T12:00:00Z",
    "status": "completed",
    "message": "What is the capital of France?"
  }
]
```

### `sub_agent_get_result`

Get the result of a background task.

**Inputs:**
- `task_id` (string, required): The task ID to get result for

**Outputs:** Task result or error if not found/expired

**Example:**
```json
{
  "task_id": "task-1234567890"
}
```

## Usage

### With a MCP Client

Configure your MCP client to use the sub-agent server:

```json
{
  "mcpServers": {
    "sub-agent": {
      "command": "./sub-agent",
      "args": [],
      "env": {
        "OPENAI_API_KEY": "your-api-key",
        "OPENAI_MODEL": "gpt-4o-mini",
        "SUB_AGENT_TTL": "4"
      }
    }
  }
}
```

### Direct API Usage

```bash
# Build and run the server
go build -o sub-agent .
./sub-agent

# Or run directly
go run main.go
```

## Architecture

The server maintains an in-memory store of background tasks with the following characteristics:

1. **Task Storage**: Concurrent map with read-write mutex for thread safety
2. **TTL Management**: Automatic cleanup of expired tasks every hour
3. **Background Processing**: Goroutines for asynchronous task execution
4. **Status Tracking**: Tasks track their status (pending, completed, expired)

## License

MIT
