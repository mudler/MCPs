# MCPS - Model Context Protocol Servers

This repository contains Model Context Protocol (MCP) servers that provide various tools and capabilities for AI models. It was mainly done to have small examples to show for [LocalAI](https://localai.io/docs/features/mcp), but works as well with any MCP client.

## Available Servers

### 🦆 DuckDuckGo Search Server

A web search server that provides search capabilities using DuckDuckGo.

**Features:**
- Web search functionality
- Configurable maximum results (default: 5)
- JSON schema validation for inputs/outputs

**Tool:**
- `search` - Search the web for information

**Configuration:**
- `MAX_RESULTS` - Environment variable to set maximum number of search results (default: 5)

**Docker Image:**
```bash
docker run -e MAX_RESULTS=10 ghcr.io/mudler/mcps/duckduckgo:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "ddg": {
          "command": "docker",
          "env": {
            "MAX_RESULTS": "10"
          },
          "args": [
            "run", "-i", "--rm", "-e", "MAX_RESULTS",
            "ghcr.io/mudler/mcps/duckduckgo:master"
          ]
        }
      }
    }
```

### 🌤️ Weather Server

A weather information server that provides current weather and forecast data for cities worldwide.

**Features:**
- Current weather conditions (temperature, wind, description)
- Multi-day weather forecast
- URL encoding for city names with special characters
- JSON schema validation for inputs/outputs
- HTTP timeout handling

**Tool:**
- `get_weather` - Get current weather and forecast for a city

**API Response Format:**
```json
{
  "temperature": "29 °C",
  "wind": "20 km/h", 
  "description": "Partly cloudy",
  "forecast": [
    {
      "day": "1",
      "temperature": "27 °C",
      "wind": "12 km/h"
    },
    {
      "day": "2", 
      "temperature": "22 °C",
      "wind": "8 km/h"
    }
  ]
}
```

**Docker Image:**
```bash
docker run ghcr.io/mudler/mcps/weather:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "weather": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/weather:master"
          ]
        }
      }
    }
```

### ⏱️ Wait Server

A simple wait/sleep server that allows AI models to autonomously wait for a specified duration. Useful for waiting for asynchronous operations to complete.

**Features:**
- Wait for a specified duration in seconds (supports fractional seconds)
- Context cancellation support for interruption
- Input validation (positive duration, maximum 1 hour)
- JSON schema validation for inputs/outputs

**Tool:**
- `wait` - Wait for a specified duration in seconds

**Input Format:**
```json
{
  "duration": 5.5
}
```

**Output Format:**
```json
{
  "message": "Waited for 5.50 seconds"
}
```

**Docker Image:**
```bash
docker run ghcr.io/mudler/mcps/wait:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "wait": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/wait:master"
          ]
        }
      }
    }
```

### 🧠 Memory Server

A persistent memory storage server that allows AI models to store, retrieve, and manage information across sessions using disk-based full-text search.

**Features:**
- Disk-based bleve index storage (no full memory load)
- Efficient full-text search across name and content fields
- Add, list, and remove memory entries
- Unique ID generation for each entry
- Timestamp tracking for entries
- Configurable storage location
- JSON schema validation for inputs/outputs
- Scalable to large numbers of entries

**Tools:**
- `add_memory` - Add a new entry to memory storage (requires both name and content)
- `list_memory` - List all memory entry names (returns only names, not full entries)
- `remove_memory` - Remove a memory entry by ID
- `search_memory` - Search memory entries by name and content using full-text search

**Configuration:**
- `MEMORY_INDEX_PATH` - Environment variable to set the bleve index path (default: `/data/memory.bleve`)

**Add Memory Input Format:**
```json
{
  "name": "User Preferences",
  "content": "User prefers coffee over tea"
}
```

**Memory Entry Format:**
```json
{
  "id": "1703123456789000000",
  "name": "User Preferences",
  "content": "User prefers coffee over tea",
  "created_at": "2023-12-21T10:30:56.789Z"
}
```

**List Memory Output Format:**
```json
{
  "names": [
    "User Preferences",
    "Meeting Notes",
    "Project Ideas"
  ],
  "count": 3
}
```

**Search Response Format:**
```json
{
  "query": "coffee",
  "results": [
    {
      "id": "1703123456789000000",
      "name": "User Preferences",
      "content": "User prefers coffee over tea",
      "created_at": "2023-12-21T10:30:56.789Z"
    }
  ],
  "count": 1
}
```

**Docker Image:**
```bash
docker run -e MEMORY_INDEX_PATH=/custom/path/memory.bleve ghcr.io/mudler/mcps/memory:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "memory": {
          "command": "docker",
          "env": {
            "MEMORY_INDEX_PATH": "/data/memory.bleve"
          },
          "args": [
            "run", "-i", "--rm", "-v", "/host/data:/data",
            "ghcr.io/mudler/mcps/memory:master"
          ]
        }
      }
    }
```

### 🏠 Home Assistant Server

A Home Assistant integration server that allows AI models to interact with and control Home Assistant entities and services.

**Features:**
- List all entities and their current states
- Get all available services with detailed information
- Call services to control devices (turn_on, turn_off, toggle, etc.)

**Tools:**
- `list_entities` - List all entities in Home Assistant
- `get_services` - Get all available services in Home Assistant
- `call_service` - Call a service in Home Assistant (e.g., turn_on, turn_off, toggle)
- `search_entities` - Search for entities by keyword (searches across entity ID, domain, state, and friendly name)
- `search_services` - Search for services by keyword (searches across service domain and name)

**Configuration:**
- `HA_TOKEN` - Home Assistant API token (required)
- `HA_HOST` - Home Assistant host URL (default: `http://localhost:8123`)

**Entity Response Format:**
```json
{
  "entities": [
    {
      "entity_id": "light.living_room",
      "state": "on",
      "friendly_name": "Living Room Light",
      "attributes": {
        "friendly_name": "Living Room Light",
        "brightness": 255
      },
      "domain": "light"
    }
  ],
  "count": 1
}
```

**Service Call Example:**
```json
{
  "domain": "light",
  "service": "turn_on",
  "entity_id": "light.living_room"
}
```

**Search Entities Example:**
```json
{
  "keyword": "living room light"
}
```

**Search Services Example:**
```json
{
  "keyword": "turn_on"
}
```

**Docker Image:**
```bash
docker run -e HA_TOKEN="your-token-here" -e HA_HOST="http://IP:PORT" ghcr.io/mudler/mcps/homeassistant:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "homeassistant": {
          "command": "docker",
          "env": {
            "HA_TOKEN": "your-home-assistant-token",
            "HA_HOST": "http://"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/homeassistant:master"
          ]
        }
      }
    }
```

### 𝕏 Twitter Server

An MCP server for interacting with Twitter/X: read tweets and profiles, search, timelines, trends, and perform actions (like, retweet, post, thread, follow) with optional media upload.

**Features:**
- Get tweets from users (with media), user profiles, search by keyword/hashtag (latest/top), rate-limited (max 50 tweets per request)
- Like/unlike, retweet/undo retweet, post tweets (text, media, reply, quote), create threads
- Home/user/mentions timelines, list tweets, trending topics (WOEID), followers/following, follow/unfollow
- Get unanswered mentions (tweets that mention you and you have not replied to, last 24 hours)
- Image upload (JPEG/PNG/GIF) for use in post_tweet/create_thread

**Tools:**
- `get_tweets` - Fetch recent tweets from a user (with media)
- `get_profile` - Get a user's profile information
- `search_tweets` - Search for tweets by hashtag or keyword
- `like_tweet` - Like or unlike a tweet
- `retweet` - Retweet or undo retweet
- `post_tweet` - Post a new tweet with optional media, reply, or quote
- `create_thread` - Create a Twitter thread
- `get_timeline` - Get tweets from home, user, or mentions timeline
- `get_unanswered_mentions` - Get tweets that mention you and you have not replied to (last 24 hours)
- `get_list_tweets` - Get tweets from a Twitter list
- `get_trends` - Get current trending topics by place (WOEID)
- `get_user_relationships` - Get followers or following list
- `follow_user` - Follow or unfollow a user
- `upload_media` - Upload an image and get media_id for post_tweet

**Configuration:**
- `TWITTER_BEARER_TOKEN` - App-only (read-only where allowed); or use OAuth 1.0a for full access
- OAuth 1.0a (required for write, home timeline, trends, media upload): `TWITTER_API_KEY`, `TWITTER_API_SECRET`, `TWITTER_ACCESS_TOKEN`, `TWITTER_ACCESS_SECRET`
- Optional: `TWITTER_MAX_TWEETS` (default 50) to cap tweets per request

**Acceptance tests:** Run with env credentials set and `TWITTER_ACCEPTANCE=true`:
```bash
TWITTER_ACCEPTANCE=true TWITTER_BEARER_TOKEN=xxx go test ./twitter/...
# Or with OAuth 1.0a for full tests:
TWITTER_ACCEPTANCE=true TWITTER_API_KEY=... TWITTER_API_SECRET=... TWITTER_ACCESS_TOKEN=... TWITTER_ACCESS_SECRET=... go test ./twitter/...
```

**Docker Image:**
```bash
docker run -e TWITTER_BEARER_TOKEN=xxx ghcr.io/mudler/mcps/twitter:latest
# Or OAuth 1.0a:
docker run -e TWITTER_API_KEY=... -e TWITTER_API_SECRET=... -e TWITTER_ACCESS_TOKEN=... -e TWITTER_ACCESS_SECRET=... ghcr.io/mudler/mcps/twitter:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "twitter": {
          "command": "docker",
          "env": {
            "TWITTER_BEARER_TOKEN": "your-bearer-token"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/twitter:master"
          ]
        }
      }
    }
```

**Note:** Twitter API access level (Free/Basic/Pro) affects rate limits and some endpoints (e.g. search). Trends and media upload use v1.1 API and require OAuth 1.0a.

### 🐚 Shell Server

A shell script execution server that allows AI models to execute shell scripts and commands.

**Features:**
- Execute shell scripts with full shell capabilities
- Configurable shell command (default: `sh -c`)
- Separate stdout and stderr capture
- Exit code reporting
- Configurable timeout (default: 30 seconds)
- JSON schema validation for inputs/outputs

**Tool:**
- `execute_command` - Execute a shell script and return the output, exit code, and any errors

**Configuration:**
- `SHELL_CMD` - Environment variable to set the shell command to use (default: `sh`). Can include arguments, e.g., `bash -x` or `zsh`

**Input Format:**
```json
{
  "script": "ls -la /tmp",
  "timeout": 30
}
```

**Output Format:**
```json
{
  "script": "ls -la /tmp",
  "stdout": "total 1234\ndrwxrwxrwt...",
  "stderr": "",
  "exit_code": 0,
  "success": true,
  "error": ""
}
```

**Docker Image:**
```bash
docker run -e SHELL_CMD=bash ghcr.io/mudler/mcps/shell:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "shell": {
          "command": "docker",
          "env": {
            "SHELL_CMD": "bash"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/shell:master"
          ]
        }
      }
    }
```

### 🔐 SSH Server

An SSH server that allows AI models to connect to remote SSH hosts and execute shell scripts.

**Features:**
- Connect to remote SSH hosts
- Execute shell scripts on remote hosts
- Support for password and key-based authentication
- Configurable remote shell command (default: `sh -c`)
- Separate stdout and stderr capture
- Exit code reporting
- Configurable timeout (default: 30 seconds)
- JSON schema validation for inputs/outputs

**Tool:**
- `execute_script` - Execute a shell script on a remote SSH host and return the output, exit code, and any errors

**Configuration:**
- `SSH_HOST` - Default SSH host (can be overridden per request)
- `SSH_PORT` - Default SSH port (default: 22)
- `SSH_USER` - Default SSH username (can be overridden per request)
- `SSH_PASSWORD` - Default SSH password (can be overridden per request, or use SSH_KEY_PATH)
- `SSH_KEY_PATH` - Path to SSH private key file (alternative to password authentication)
- `SSH_KEY_PASSPHRASE` - Passphrase for encrypted SSH private key (if needed)
- `SSH_SHELL_CMD` - Remote shell command to use (default: `sh -c`)

**Input Format:**
```json
{
  "host": "example.com",
  "port": 22,
  "user": "username",
  "password": "password",
  "script": "ls -la /tmp",
  "timeout": 30
}
```

Or using key-based authentication:
```json
{
  "host": "example.com",
  "user": "username",
  "key_path": "/path/to/private/key",
  "script": "ls -la /tmp",
  "timeout": 30
}
```

**Output Format:**
```json
{
  "host": "example.com",
  "script": "ls -la /tmp",
  "stdout": "total 1234\ndrwxrwxrwt...",
  "stderr": "",
  "exit_code": 0,
  "success": true,
  "error": ""
}
```

**Docker Image:**
```bash
docker run -e SSH_HOST=example.com -e SSH_USER=user -e SSH_PASSWORD=pass ghcr.io/mudler/mcps/ssh:latest
```

Or with key-based authentication:
```bash
docker run -e SSH_HOST=example.com -e SSH_USER=user -e SSH_KEY_PATH=/path/to/key -v /host/keys:/path/to/key ghcr.io/mudler/mcps/ssh:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "ssh": {
          "command": "docker",
          "env": {
            "SSH_HOST": "example.com",
            "SSH_USER": "username",
            "SSH_PASSWORD": "password",
            "SSH_SHELL_CMD": "bash -c"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/ssh:master"
          ]
        }
      }
    }
```

### 🔧 Script Runner Server

A flexible script and program execution server that allows AI models to run pre-defined scripts and programs as tools. Scripts can be defined inline or via file paths, and programs can be executed directly.

**Features:**
- Execute scripts from file paths or inline content
- Run arbitrary programs/commands
- Automatic interpreter detection (shebang or file extension)
- Configurable timeouts per script/program
- Custom working directories and environment variables
- Comprehensive output capture (stdout, stderr, exit code, duration)

**Configuration:**
- `SCRIPTS` - JSON string defining scripts/programs (required)

**Script Configuration Format:**
```json
[
  {
    "name": "hello_world",
    "description": "A simple hello world script",
    "content": "#!/bin/bash\necho 'Hello, World!'",
    "timeout": 10
  },
  {
    "name": "run_python",
    "description": "Run a Python script from file",
    "path": "/scripts/process_data.py",
    "interpreter": "python3",
    "timeout": 30,
    "working_dir": "/data"
  },
  {
    "name": "list_files",
    "description": "List files in a directory",
    "command": "ls",
    "timeout": 5
  }
]
```

**Executor Object Fields:**
- `name` (string, required): Tool name (must be valid identifier)
- `description` (string, required): Tool description
- `content` (string, optional): Inline script content (mutually exclusive with `path` and `command`)
- `path` (string, optional): Path to script file (mutually exclusive with `content` and `command`)
- `command` (string, optional): Command/program to execute (mutually exclusive with `content` and `path`)
- `interpreter` (string, optional): Interpreter to use (default: auto-detect from shebang or file extension)
- `timeout` (int, optional): Timeout in seconds (default: 30)
- `working_dir` (string, optional): Working directory for execution
- `env` (map[string]string, optional): Additional environment variables

**Execution Input:**
```json
{
  "args": ["arg1", "arg2"]
}
```

**Execution Output:**
```json
{
  "stdout": "Hello, World!\n",
  "stderr": "",
  "exit_code": 0,
  "duration_ms": 15
}
```

**Docker Image:**
```bash
docker run -e SCRIPTS='[{"name":"hello","description":"Hello script","content":"#!/bin/bash\necho hello"}]' ghcr.io/mudler/mcps/scripts:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "scripts": {
          "command": "docker",
          "env": {
            "SCRIPTS": "[{\"name\":\"hello\",\"description\":\"Hello script\",\"content\":\"#!/bin/bash\\necho hello\"},{\"name\":\"list_files\",\"description\":\"List files\",\"command\":\"ls\"}]"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/scripts:master"
          ]
        }
      }
    }
```

### 📚 LocalRecall Server

A knowledge base management server that provides tools to interact with [LocalRecall](https://github.com/mudler/LocalRecall)'s REST API for managing collections, searching content, and managing documents.

**Features:**
- Search content in collections
- Create and reset collections
- Add documents to collections
- List collections and files
- Delete entries from collections
- Configurable tool enablement for security

**Tools:**
- `search` - Search content in a LocalRecall collection
- `create_collection` - Create a new collection
- `reset_collection` - Reset (clear) a collection
- `add_document` - Add a document to a collection
- `list_collections` - List all collections
- `list_files` - List files in a collection
- `delete_entry` - Delete an entry from a collection

**Configuration:**
- `LOCALRECALL_URL` - Base URL for LocalRecall API (default: `http://localhost:8080`)
- `LOCALRECALL_API_KEY` - Optional API key for authentication (sent as `Authorization: Bearer <key>`)
- `LOCALRECALL_COLLECTION` - Default collection name (if set, tools are registered without `collection_name` parameter - the collection is automatically used from the environment variable)
- `LOCALRECALL_ENABLED_TOOLS` - Comma-separated list of tools to enable (default: all tools enabled). Valid values: `search`, `create_collection`, `reset_collection`, `add_document`, `list_collections`, `list_files`, `delete_entry`

**Note:** When `LOCALRECALL_COLLECTION` is set, the tools `search`, `add_document`, `list_files`, and `delete_entry` are registered with different input schemas that do not include the `collection_name` parameter. The collection name is automatically taken from the environment variable.

**Search Input Format:**

When `LOCALRECALL_COLLECTION` is **not** set:
```json
{
  "collection_name": "myCollection",
  "query": "search term",
  "max_results": 5
}
```

When `LOCALRECALL_COLLECTION` is set (e.g., `LOCALRECALL_COLLECTION=myCollection`), the tool schema does not include `collection_name`:
```json
{
  "query": "search term",
  "max_results": 5
}
```

**Search Output Format:**
```json
{
  "query": "search term",
  "max_results": 5,
  "results": [
    {
      "content": "...",
      "metadata": {...}
    }
  ],
  "count": 1
}
```

**Add Document Input Format:**

When `LOCALRECALL_COLLECTION` is **not** set:
```json
{
  "collection_name": "myCollection",
  "file_path": "/path/to/file.txt",
  "filename": "file.txt"
}
```

Or with inline content:
```json
{
  "collection_name": "myCollection",
  "file_content": "Document content here",
  "filename": "document.txt"
}
```

When `LOCALRECALL_COLLECTION` is set, the tool schema does not include `collection_name`:
```json
{
  "file_path": "/path/to/file.txt",
  "filename": "file.txt"
}
```

**List Files Input Format:**

When `LOCALRECALL_COLLECTION` is **not** set:
```json
{
  "collection_name": "myCollection"
}
```

When `LOCALRECALL_COLLECTION` is set, the tool schema has no parameters (empty object):
```json
{}
```

**Delete Entry Input Format:**

When `LOCALRECALL_COLLECTION` is **not** set:
```json
{
  "collection_name": "myCollection",
  "entry": "filename.txt"
}
```

When `LOCALRECALL_COLLECTION` is set, the tool schema does not include `collection_name`:
```json
{
  "entry": "filename.txt"
}
```

**Docker Image:**
```bash
docker run -e LOCALRECALL_URL=http://localhost:8080 -e LOCALRECALL_API_KEY=your-key-here ghcr.io/mudler/mcps/localrecall:latest
```

**With default collection (tools will not require `collection_name` parameter):**
```bash
docker run -e LOCALRECALL_URL=http://localhost:8080 -e LOCALRECALL_COLLECTION=myCollection ghcr.io/mudler/mcps/localrecall:latest
```

When `LOCALRECALL_COLLECTION` is set, the collection-specific tools (`search`, `add_document`, `list_files`, `delete_entry`) are automatically configured to use that collection, and the `collection_name` parameter is removed from their input schemas.

**Enable specific tools only:**
```bash
docker run -e LOCALRECALL_URL=http://localhost:8080 -e LOCALRECALL_ENABLED_TOOLS="search,list_collections,list_files" ghcr.io/mudler/mcps/localrecall:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "localrecall": {
          "command": "docker",
          "env": {
            "LOCALRECALL_URL": "http://localhost:8080",
            "LOCALRECALL_API_KEY": "your-api-key",
            "LOCALRECALL_COLLECTION": "myCollection",
            "LOCALRECALL_ENABLED_TOOLS": "search,list_collections,add_document"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/localrecall:master"
          ]
        }
      }
    }
```

### ✅ TODO Server

A shared TODO list management server that allows multiple agents to coordinate tasks with states, assignees, and dependencies. Perfect for agent team coordination with dependency management and role-based access control.

**Features:**
- Shared TODO list accessible by multiple agents/processes
- File-based persistence with atomic writes
- File locking for concurrent access safety
- Task states: pending, in_progress, done
- Assignee tracking for task ownership
- **Dependency management** - TODOs can depend on other TODOs
- **Circular dependency detection** - Prevents invalid dependency chains
- **Status validation** - Prevents starting/completing TODOs until dependencies are satisfied
- **Role-based access control** - Admin mode for leader actions, agent mode for self-service
- Full CRUD operations (add, update, remove, list)
- Status summary with counts by state and assignee
- Query ready and blocked TODOs

**Tools:**

**Always Available (Agent & Admin):**
- `list_todos` - List all TODO items
- `get_todo_status` - Get a summary of the TODO list with counts by status and assignee
- `get_ready_todos` - Get all TODO items that are ready to start (pending with all dependencies satisfied)
- `get_blocked_todos` - Get all TODO items that are blocked by dependencies
- `get_todo_dependencies` - Get dependencies for a TODO item (direct and optionally transitive)
- `update_todo_status` - Update the status of a TODO item (pending, in_progress, or done)
  - In agent mode: Only allows updating TODOs assigned to the agent (requires `agent_name` parameter)
  - In admin mode: Allows updating any TODO (no `agent_name` required)

**Admin Only (requires `TODO_ADMIN_MODE=true`):**
- `add_todo` - Add a new TODO item to the shared list
- `remove_todo` - Remove a TODO item by ID
- `update_todo_assignee` - Update the assignee of a TODO item
- `add_todo_dependency` - Add a dependency to a TODO item
- `remove_todo_dependency` - Remove a dependency from a TODO item

**Configuration:**
- `TODO_FILE_PATH` - Environment variable to set the TODO file path (default: `/data/todos.json`)
- `TODO_ADMIN_MODE` - Set to `true` to enable admin-only tools (add, remove, assign, manage dependencies). When not set, only read operations and self-service status updates are available.

**TODO Item Format:**
```json
{
  "id": "task-1",
  "title": "Implement feature X",
  "status": "in_progress",
  "assignee": "agent1",
  "depends_on": ["task-0"]
}
```

**Add TODO Input Format:**
```json
{
  "id": "task-1",
  "title": "Implement feature X",
  "assignee": "agent1",
  "depends_on": ["task-0"]
}
```

**Note:** The `id` field is **required** and must be unique. IDs are not auto-generated for predictability.

**Update Status Input Format:**

In admin mode:
```json
{
  "id": "task-1",
  "status": "done"
}
```

In agent mode (requires `agent_name`):
```json
{
  "id": "task-1",
  "status": "done",
  "agent_name": "agent1"
}
```

**Dependency Management:**

TODOs can depend on other TODOs. A TODO cannot transition to `in_progress` or `done` until all its dependencies are `done`. The system prevents:
- Circular dependencies (A → B → A)
- Starting/completing TODOs with unsatisfied dependencies
- Removing TODOs that other TODOs depend on

**Get Ready TODOs Output Format:**
```json
{
  "items": [
    {
      "id": "task-2",
      "title": "Task 2",
      "status": "pending",
      "assignee": "agent1",
      "depends_on": ["task-1"]
    }
  ],
  "count": 1
}
```

**Get Blocked TODOs Output Format:**
```json
{
  "items": [
    {
      "id": "task-3",
      "title": "Task 3",
      "status": "pending",
      "assignee": "agent2",
      "blocked_by": [
        {
          "id": "task-1",
          "title": "Task 1",
          "status": "pending"
        }
      ]
    }
  ],
  "count": 1
}
```

**Get TODO Dependencies Output Format:**
```json
{
  "direct": [
    {
      "id": "task-1",
      "title": "Task 1",
      "status": "done"
    }
  ],
  "direct_count": 1
}
```

**List TODOs Output Format:**
```json
{
  "items": [
    {
      "id": "task-1",
      "title": "Implement feature X",
      "status": "in_progress",
      "assignee": "agent1",
      "depends_on": ["task-0"]
    }
  ],
  "count": 1
}
```

**Status Summary Output Format:**
```json
{
  "total": 10,
  "pending": 3,
  "in_progress": 5,
  "done": 2,
  "blocked": 2,
  "ready": 1,
  "by_assignee": {
    "agent1": 4,
    "agent2": 6
  }
}
```

**Docker Image:**

Agent mode (read-only + self-service):
```bash
docker run -e TODO_FILE_PATH=/custom/path/todos.json -v /host/data:/data ghcr.io/mudler/mcps/todo:latest
```

Admin mode (full access):
```bash
docker run -e TODO_FILE_PATH=/custom/path/todos.json -e TODO_ADMIN_MODE=true -v /host/data:/data ghcr.io/mudler/mcps/todo:latest
```

**LocalAI configuration (to add to the model config):**

Agent mode:
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "todo": {
          "command": "docker",
          "env": {
            "TODO_FILE_PATH": "/data/todos.json"
          },
          "args": [
            "run", "-i", "--rm", "-v", "/host/data:/data",
            "ghcr.io/mudler/mcps/todo:master"
          ]
        }
      }
    }
```

Admin mode:
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "todo": {
          "command": "docker",
          "env": {
            "TODO_FILE_PATH": "/data/todos.json",
            "TODO_ADMIN_MODE": "true"
          },
          "args": [
            "run", "-i", "--rm", "-v", "/host/data:/data",
            "ghcr.io/mudler/mcps/todo:master"
          ]
        }
      }
    }
```

**Note:** When `TODO_ADMIN_MODE` is not set or set to `false`, only read operations and self-service status updates are available. Agents can only update TODOs assigned to them by providing their `agent_name` in the `update_todo_status` request. Admin mode enables all tools including adding/removing TODOs, managing dependencies, and assigning tasks.

### 📬 Mailbox Server

A shared mailbox server that enables message exchange between different agents in a team. Each agent instance reads only its own messages while being able to send messages to any agent.

**Features:**
- Shared mailbox accessible by multiple agents/processes
- File-based persistence with atomic writes
- File locking for concurrent access safety
- Agent-specific message filtering
- Read/unread status tracking
- Message deletion (only by recipient)
- Timestamp tracking for all messages

**Tools:**
- `send_message` - Send a message to a recipient agent
- `read_messages` - Read all messages for this agent
- `mark_message_read` - Mark a message as read by ID
- `mark_message_unread` - Mark a message as unread by ID
- `delete_message` - Delete a message by ID (only if recipient matches this agent)

**Configuration:**
- `MAILBOX_FILE_PATH` - Environment variable to set the mailbox file path (default: `/data/mailbox.json`)
- `MAILBOX_AGENT_NAME` - Environment variable for this agent's name (required)

**Message Format:**
```json
{
  "id": "1703123456789000000",
  "sender": "agent1",
  "recipient": "agent2",
  "content": "Please review the changes",
  "timestamp": "2023-12-21T10:30:56.789Z",
  "read": false
}
```

**Send Message Input Format:**
```json
{
  "recipient": "agent2",
  "content": "Please review the changes"
}
```

**Send Message Output Format:**
```json
{
  "id": "1703123456789000000",
  "sender": "agent1",
  "recipient": "agent2",
  "content": "Please review the changes",
  "timestamp": "2023-12-21T10:30:56.789Z"
}
```

**Read Messages Output Format:**
```json
{
  "messages": [
    {
      "id": "1703123456789000000",
      "sender": "agent1",
      "recipient": "agent2",
      "content": "Please review the changes",
      "timestamp": "2023-12-21T10:30:56.789Z",
      "read": false
    }
  ],
  "count": 1,
  "unread": 1
}
```

**Docker Image:**
```bash
docker run -e MAILBOX_FILE_PATH=/custom/path/mailbox.json -e MAILBOX_AGENT_NAME=agent1 -v /host/data:/data ghcr.io/mudler/mcps/mailbox:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "mailbox": {
          "command": "docker",
          "env": {
            "MAILBOX_FILE_PATH": "/data/mailbox.json",
            "MAILBOX_AGENT_NAME": "agent1"
          },
          "args": [
            "run", "-i", "--rm", "-v", "/host/data:/data",
            "ghcr.io/mudler/mcps/mailbox:master"
          ]
        }
      }
    }
```

**Note:** Each agent instance must have a unique `MAILBOX_AGENT_NAME` to properly filter and manage its own messages. The mailbox file is shared across all agents, but each agent only sees messages where it is the recipient.

## Development

### Prerequisites

- Go 1.24.7 or later
- Docker (for containerized builds)
- Make (for using the Makefile)

### Building

Use the provided Makefile for easy development:

```bash
# Show all available commands
make help

# Development workflow
make dev

# Build specific server
make MCP_SERVER=duckduckgo build
make MCP_SERVER=weather build
make MCP_SERVER=wait build
make MCP_SERVER=memory build
make MCP_SERVER=shell build
make MCP_SERVER=ssh build
make MCP_SERVER=scripts build
make MCP_SERVER=localrecall build
make MCP_SERVER=todo build
make MCP_SERVER=mailbox build

# Run tests and checks
make ci-local

# Build multi-architecture images
make build-multiarch
```

### Adding New Servers

To add a new MCP server:

1. Create a new directory under the project root
2. Implement the server following the MCP SDK patterns
3. Update the GitHub Actions workflow matrix in `.github/workflows/image.yml`
4. Update this README with the new server information

Example server structure:
```go
package main

import (
    "context"
    "log"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
    server := mcp.NewServer(&mcp.Implementation{
        Name: "your-server", 
        Version: "v1.0.0"
    }, nil)
    
    // Add your tools here
    mcp.AddTool(server, &mcp.Tool{
        Name: "your-tool", 
        Description: "your tool description"
    }, YourToolFunction)
    
    if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
        log.Fatal(err)
    }
}
```

## Docker Images

Docker images are automatically built and pushed to GitHub Container Registry:

- `ghcr.io/mudler/mcps/duckduckgo:latest` - Latest DuckDuckGo server
- `ghcr.io/mudler/mcps/duckduckgo:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/duckduckgo:master` - Development versions
- `ghcr.io/mudler/mcps/weather:latest` - Latest Weather server
- `ghcr.io/mudler/mcps/weather:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/weather:master` - Development versions
- `ghcr.io/mudler/mcps/wait:latest` - Latest Wait server
- `ghcr.io/mudler/mcps/wait:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/wait:master` - Development versions
- `ghcr.io/mudler/mcps/memory:latest` - Latest Memory server
- `ghcr.io/mudler/mcps/memory:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/memory:master` - Development versions
- `ghcr.io/mudler/mcps/shell:latest` - Latest Shell server
- `ghcr.io/mudler/mcps/shell:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/shell:master` - Development versions
- `ghcr.io/mudler/mcps/ssh:latest` - Latest SSH server
- `ghcr.io/mudler/mcps/ssh:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/ssh:master` - Development versions
- `ghcr.io/mudler/mcps/homeassistant:latest` - Latest Home Assistant server
- `ghcr.io/mudler/mcps/homeassistant:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/homeassistant:master` - Development versions
- `ghcr.io/mudler/mcps/scripts:latest` - Latest Script Runner server
- `ghcr.io/mudler/mcps/scripts:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/scripts:master` - Development versions
- `ghcr.io/mudler/mcps/localrecall:latest` - Latest LocalRecall server
- `ghcr.io/mudler/mcps/localrecall:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/localrecall:master` - Development versions
- `ghcr.io/mudler/mcps/todo:latest` - Latest TODO server
- `ghcr.io/mudler/mcps/todo:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/todo:master` - Development versions
- `ghcr.io/mudler/mcps/mailbox:latest` - Latest Mailbox server
- `ghcr.io/mudler/mcps/mailbox:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/mailbox:master` - Development versions

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Run `make ci-local` to ensure all checks pass
6. Submit a pull request

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Model Context Protocol

This project implements servers for the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/), a standard for connecting AI models to external data sources and tools.

For more information about MCP, visit the [official documentation](https://modelcontextprotocol.io/docs).
