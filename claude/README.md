# Claude MCP Server

This is an MCP (Model Context Protocol) server that provides integration with Claude Code CLI, allowing other MCP clients to delegate tasks to Claude Code in the background. It follows the same pattern as the opencode MCP server but is specifically designed for Claude Code.

## Features

- **Background Task Delegation**: Start Claude Code sessions in the background and monitor their status
- **Session Management**: Create, monitor, stop, and list sessions
- **Log Access**: Retrieve stdout and stderr logs from running or completed sessions
- **Tool Restrictions**: Support for `--allowedTools` and `--tools` flags to control Claude's capabilities
- **Environment Passthrough**: All OS environment variables are passed to Claude subprocesses
- **Configurable**: Extensive environment variables for customization

## Usage

### Adding the Server

Add this MCP server to your Claude Code configuration:

```bash
claude mcp add claude-mcp -- npx -y mudler/MCPs/claude
```

Or use the Docker-based approach if you've built the Docker image:

```bash
claude mcp add claude-mcp -- docker run -i --rm claude-mcp-server
```

### Starting a Session

Use the `start_session` tool to begin a new Claude session:

```json
{
  "message": "Find and fix the bug in auth.py",
  "allowed_tools": "Bash,Read,Edit",
  "title": "Bug Fix Task"
}
```

This returns a session ID that can be used to monitor progress.

### Checking Session Status

Use the `get_session_status` tool with the session ID to check if the session is:
- `starting` - Session is initializing
- `running` - Claude is actively working
- `completed` - Task finished successfully (exit code 0)
- `failed` - Task failed with non-zero exit code
- `stopped` - Session was manually stopped
- `not_found` - Session ID doesn't exist

### Retrieving Logs

Use the `get_session_logs` tool to see what Claude is doing:
- Specify the session ID
- Optionally specify number of lines to retrieve (default 100)
- Returns both stdout and stderr

### Stopping a Session

Use the `stop_session` tool to terminate a running session. Use `force=true` to kill immediately.

### Listing Sessions

Use the `list_sessions` tool to see all sessions, optionally filtered by status.

## Environment Variables

### Server Configuration

- `CLAUDE_SESSION_DIR`: Directory for session state (default: `/tmp/claude-sessions`)
- `CLAUDE_MAX_SESSIONS`: Maximum concurrent sessions (default: `10`)
- `CLAUDE_LOG_RETENTION_HOURS`: Hours to keep logs before cleanup (default: `24`)
- `CLAUDE_WORK_DIR`: Working directory for Claude processes (default: `/root`)

### Authentication

- `CLAUDE_CREDENTIALS`: JSON content of `~/.claude/.credentials.json`. On startup, the server writes this to disk so Claude CLI picks up OAuth tokens. Useful for containers and remote environments.
- `ANTHROPIC_API_KEY`: Anthropic API key, passed through to Claude subprocesses automatically. Alternative to OAuth credentials.

### Claude Code Invocation

- `CLAUDE_BINARY`: Path to Claude binary (default: `claude`)
- `CLAUDE_MODEL`: Default model to use (e.g., `sonnet`, `opus`)
- `CLAUDE_AGENT`: Specify an agent for sessions
- `CLAUDE_FORMAT`: Output format (default: `json`)
- `CLAUDE_SHARE`: Whether to share sessions (`true`/`false`)
- `CLAUDE_ATTACH`: Files to attach to sessions
- `CLAUDE_PORT`: Port for remote control
- `CLAUDE_VARIANT`: Model variant
- `CLAUDE_ALLOWED_TOOLS`: Tools allowed without prompting (e.g., `"Bash,Read,Edit"`)
- `CLAUDE_TOOLS`: Restrict which tools Claude can use (e.g., `"Bash,Read,Edit"`)
- `CLAUDE_DANGEROUSLY_SKIP_PERMISSIONS`: Set to `true` to pass `--dangerously-skip-permissions` to Claude, allowing all tools without any permission prompts

### Tool Naming (Optional)

- `CLAUDE_TOOL_START_SESSION_NAME`: Override the start_session tool name
- `CLAUDE_TOOL_GET_SESSION_STATUS_NAME`: Override the get_session_status tool name
- `CLAUDE_TOOL_GET_SESSION_LOGS_NAME`: Override the get_session_logs tool name
- `CLAUDE_TOOL_STOP_SESSION_NAME`: Override the stop_session tool name
- `CLAUDE_TOOL_LIST_SESSIONS_NAME`: Override the list_sessions tool name

## Integration with MCPs Repository

This server is designed to be part of the [MCPs](https://github.com/mudler/MCPs) collection. It follows the same structure and patterns as other MCP servers in that repository.

### Directory Structure

```
claude/
├── main.go          # MCP server setup and tool registration
├── session.go       # Session management logic
├── handlers.go      # Tool handlers (input/output types)
├── Dockerfile       # Container image definition
└── README.md        # This file
```

### Building

The server can be built using the Dockerfile or directly with Go:

```bash
go build -o claude-mcp-server ./claude
```

### Testing

You can test the server directly:

```bash
# Run the server (it will wait for MCP input on stdin/stdout)
./claude-mcp-server
```

Or test with the MCP inspector:

```bash
npx @modelcontextprotocol/inspector ./claude-mcp-server
```

## Security Considerations

- The server runs Claude Code with the permissions of the user running the MCP client
- Environment variables are passed through to Claude subprocesses
- Tool restrictions (`--allowedTools`, `--tools`) should be used to limit Claude's capabilities
- Session logs are stored in the session directory and cleaned up based on retention policy
- Consider using `CLAUDE_MAX_SESSIONS` to limit resource usage

## Example Workflow

1. **Start a session** to fix a bug:
   ```
   start_session(message="Fix the authentication bug in login.go", allowed_tools="Bash,Read,Edit")
   ```

2. **Check status** after a few seconds:
   ```
   get_session_status(session_id="abc123...")
   ```

3. **Get logs** to see progress:
   ```
   get_session_logs(session_id="abc123...", lines=50)
   ```

4. **Stop if needed** (if it's taking too long or going wrong):
   ```
   stop_session(session_id="abc123...", force=false)
   ```

5. **List all sessions** to overview:
   ```
   list_sessions(status_filter="running")
   ```

## Troubleshooting

### Sessions fail to start
- Check that `CLAUDE_BINARY` points to a valid Claude executable
- Verify the Claude binary is in PATH or specify full path
- Check session logs for error messages
- Ensure environment variables are properly set

### No tools available
- Check `CLAUDE_ALLOWED_TOOLS` and `CLAUDE_TOOLS` environment variables
- Remember that `--allowedTools` enables tools without prompting, while `--tools` restricts which tools are available
- Use `claude --help` to see available tool names

### High resource usage
- Reduce `CLAUDE_MAX_SESSIONS` to limit concurrent sessions
- Set `CLAUDE_LOG_RETENTION_HOURS` to a lower value for faster cleanup
- Use `stop_session` to terminate unused sessions

## License

This MCP server is part of the MCPs repository and follows the same license (MIT).
