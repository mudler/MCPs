# Sub-Agent MCP Server

An MCP server implementation for managing autonomous sub-agent jobs with OpenAI chat completion capabilities.

## Features

- **Job Management**: Create, retrieve, list, and update sub-agent jobs with status tracking
- **TTL Support**: Jobs automatically expire based on configurable time-to-live
- **OpenAI Integration**: Built-in chat completion tool using OpenAI API
- **Environment Configuration**: Flexible configuration via environment variables

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_BASE_URL` | OpenAI API endpoint | `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Default model for chat completions | `gpt-4o-mini` |
| `OPENAI_API_KEY` | API key for OpenAI | Required |
| `SUB_AGENT_TTL` | Default job TTL duration | `24h` |

## MCP Tools

### `create_job`
Create a new sub-agent job with optional inputs.

**Input:**
- `id` (string): Unique job identifier
- `inputs` (object, optional): Input parameters for the job

**Output:**
- `job_id`: The created job ID
- `status`: Initial status (pending)

### `get_job`
Retrieve a job by its ID.

**Input:**
- `id` (string): Job identifier

**Output:**
- Full job details including status, inputs, result, and timestamps

### `list_jobs`
List all sub-agent jobs, optionally filtered by status.

**Input:**
- `status` (string, optional): Filter by status (pending, running, completed, failed)

**Output:**
- Array of jobs (automatically excludes jobs older than their TTL)

### `update_job`
Update a job's status and result.

**Input:**
- `id` (string): Job identifier
- `status` (string): New status (running, completed, failed)
- `result` (string, optional): Job result
- `error` (string, optional): Error message if failed

**Output:**
- `job_id`: The updated job ID
- `status`: Updated status

### `chat_completion`
Send a chat completion request to OpenAI API.

**Input:**
- `messages` (array): Array of message objects with `role` and `content`
- `model` (string, optional): Model to use (defaults to OPENAI_MODEL env var)

**Output:**
- `content`: The AI response content
- `error` (string, optional): Error message if request failed

## Building

```bash
cd sub-agent
go build -o sub-agent .
```

## Running

```bash
./sub-agent
```

The server runs via stdio transport, suitable for use with MCP clients.

## Job Lifecycle

1. **pending**: Job created but not yet started
2. **running**: Job is currently executing
3. **completed**: Job finished successfully with a result
4. **failed**: Job failed with an error

Jobs are automatically excluded from listings once they exceed their TTL.
