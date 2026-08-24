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

### 🤖 Codemogger Server

A codemogger MCP server that provides code analysis and indexing capabilities.

**Features:**
- Code search functionality
- Code indexing capabilities
- Reindex functionality
- JSON schema validation for inputs/outputs

**Tools:**
- `codemogger_search` - Search code using codemogger
- `codemogger_index` - Index code using codemogger
- `codemogger_reindex` - Reindex code using codemogger

**Docker Image:**
```bash
docker run ghcr.io/mudler/mcps/codemogger:latest
```

**LocalAI configuration ( to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "codemogger": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/codemogger:master"
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


### OpenWeatherMap Server

An OpenWeatherMap-based weather server that provides current conditions and a 5-day forecast for cities worldwide.

**Features:**
- OpenWeatherMap geocoding lookup for city names
- Current weather conditions (temperature, wind, description)
- 5-day forecast derived from the OpenWeatherMap forecast API
- Retry logic for common city formats such as `City, TX`
- JSON schema validation for inputs/outputs
- HTTP timeout and upstream error handling

**Tool:**
- `get_weather` - Get current weather and a 5-day forecast for a city using OpenWeatherMap

**Configuration:**
- `OWM_API_KEY` - OpenWeatherMap API key

**API Response Format:**
```json
{
  "temperature": "80.1 F",
  "wind": "17.3 mph",
  "description": "broken clouds",
  "forecast": [
    {
      "day": "2026-04-03",
      "temperature": "80.1 F",
      "wind": "17.3 mph"
    },
    {
      "day": "2026-04-04",
      "temperature": "76.2 F",
      "wind": "11.4 mph"
    }
  ]
}
```

**Docker Image:**
```bash
docker run -e OWM_API_KEY=your-api-key ghcr.io/mudler/mcps/openweathermap:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "openweathermap": {
          "command": "docker",
          "env": {
            "OWM_API_KEY": "your-api-key"
          },
          "args": [
            "run", "-i", "--rm", "--init",
            "-e", "OWM_API_KEY",
            "ghcr.io/mudler/mcps/openweathermap:master"
          ]
        }
      }
    }
```

### 🧠 Think Server

A no-op tool that forces the model to think about a message. Useful for debugging or forcing explicit reasoning steps in the model.

**Features:**
- Simple message input that gets echoed back
- Forces the model to explicitly process and think about the message
- Input validation (non-empty message)
- JSON schema validation for inputs/outputs

**Tool:**
- `think` - Think about a given message

**Input Format:**
```json
{
  "message": "What is the capital of France?"
}
```

**Output Format:**
```json
{
  "result": "Thinking about: What is the capital of France?"
}
```

**Docker Image:**
```bash
docker run ghcr.io/mudler/mcps/think:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "think": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/think:master"
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
- `MEMORY_ADD_TOOL_NAME` - Environment variable to override the name of the add memory tool (default: `add_memory`)
- `MEMORY_LIST_TOOL_NAME` - Environment variable to override the name of the list memory tool (default: `list_memory`)
- `MEMORY_REMOVE_TOOL_NAME` - Environment variable to override the name of the remove memory tool (default: `remove_memory`)
- `MEMORY_SEARCH_TOOL_NAME` - Environment variable to override the name of the search memory tool (default: `search_memory`)

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
# Basic usage with default tool names
docker run -e MEMORY_INDEX_PATH=/custom/path/memory.bleve ghcr.io/mudler/mcps/memory:latest

# Usage with custom tool names
docker run -e MEMORY_INDEX_PATH=/custom/path/memory.bleve \
  -e MEMORY_ADD_TOOL_NAME=store \
  -e MEMORY_LIST_TOOL_NAME=list \
  -e MEMORY_REMOVE_TOOL_NAME=delete \
  -e MEMORY_SEARCH_TOOL_NAME=find \
  ghcr.io/mudler/mcps/memory:latest
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
            "MEMORY_INDEX_PATH": "/data/memory.bleve",
            "MEMORY_ADD_TOOL_NAME": "store",
            "MEMORY_LIST_TOOL_NAME": "list",
            "MEMORY_REMOVE_TOOL_NAME": "delete",
            "MEMORY_SEARCH_TOOL_NAME": "find"
          },
          "args": [
            "run", "-i", "--rm", "-v", "/host/data:/data",
            "-e", "MEMORY_INDEX_PATH",
            "-e", "MEMORY_ADD_TOOL_NAME",
            "-e", "MEMORY_LIST_TOOL_NAME",
            "-e", "MEMORY_REMOVE_TOOL_NAME",
            "-e", "MEMORY_SEARCH_TOOL_NAME",
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
- `SHELL_CMD` - Environment variable to set the shell command to use (default: `sh -c`). Can include arguments, e.g., `bash -x` or `zsh`
- `SHELL_TIMEOUT_DISABLED` - Set to `true` to disable the timeout completely (default: timeout is 30 seconds)
- `SHELL_TIMEOUT` - Environment variable to set the timeout in seconds (default: 30 seconds)
- `SHELL_WORKING_DIR` - Environment variable to set the working directory for script execution (default: current directory)

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
**Docker Image:**
```bash
docker run -e SHELL_CMD=bash ghcr.io/mudler/mcps/shell:latest
```

With timeout disabled:
```bash
docker run -e SHELL_CMD=bash -e SHELL_TIMEOUT_DISABLED=true ghcr.io/mudler/mcps/shell:latest
```

With custom working directory:
```bash
docker run -e SHELL_CMD=bash -e SHELL_WORKING_DIR=/workspace ghcr.io/mudler/mcps/shell:latest
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
            "SHELL_CMD": "bash",
            "SHELL_WORKING_DIR": "/workspace"
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

### 📁 Filesystem Server

A filesystem operations server that provides tools to read, write, edit, and search files and directories.

**Features:**
- Read files with line numbers and optional offset/limit
- Write files with automatic parent directory creation
- Edit files with string replacement (single or all occurrences)
- Find files by glob patterns (sorted by modification time)
- Search file contents with regex patterns
- JSON schema validation for inputs/outputs

**Tools:**
- `read` - Read file with line numbers, supports optional offset and limit for reading specific line ranges
- `write` - Write content to a file, creates parent directories if needed, overwrites existing files
- `edit` - Replace old string with new string in a file, old string must be unique unless all=true
- `glob` - Find files by glob pattern, sorted by modification time (newest first)
- `grep` - Search files for regex pattern, returns up to 50 matches

**Read File Input Format:**
```json
{
  "path": "/path/to/file.txt",
  "offset": 0,
  "limit": 50
}
```

**Read File Output Format:**
```json
{
  "content": "   1| line one\n   2| line two",
  "total_lines": 100,
  "success": true
}
```

**Write File Input Format:**
```json
{
  "path": "/path/to/file.txt",
  "content": "file content here"
}
```

**Edit File Input Format:**
```json
{
  "path": "/path/to/file.txt",
  "old": "old text",
  "new": "new text",
  "all": false
}
```

**Glob Files Input Format:**
```json
{
  "pat": "**/*.go",
  "path": "."
}
```

**Grep Files Input Format:**
```json
{
  "pat": "func main",
  "path": "."
}
```

**Docker Image:**
```bash
docker run -v /host/workspace:/workspace ghcr.io/mudler/mcps/filesystem:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "filesystem": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm", "-v", "/host/workspace:/workspace",
            "ghcr.io/mudler/mcps/filesystem:master"
          ]
        }
      }
    }
```

### 🗄️ Samba Server

An SMB2/SMB3 server that talks to a Windows or Samba share over the network. It connects to the share directly, so no `mount`, no `cifs-utils` and no privileged container are needed.

**Features:**
- Native SMB2/SMB3 client, no host mount required
- Every path is relative to the share root and cannot escape it
- Name-based recursive search that never downloads file contents
- Reads refuse binary files and anything above a configurable size limit
- Two independent safety switches: read-only, and deletion disabled
- JSON schema validation for inputs/outputs

**Tools:**
- `samba_list` - List the files and directories directly inside a directory on the share
- `samba_search` - Find files and directories by name, matching a glob such as `*.gguf` or plain text as a substring
- `samba_read` - Read a text file with line numbers, refusing binary and oversized files
- `samba_write` - Write content to a file, creating parent directories as needed
- `samba_move` - Move or rename a file or directory within the share
- `samba_delete` - Delete a file or directory, recursing only when asked

The `samba_` prefix keeps these names clear of the filesystem server, which also registers `read` and `write`. Set `SMB_TOOL_PREFIX` to change it, for example `nas1_` when two shares are connected at once, or to an empty value for bare names.

**Configuration:**

| Variable | Default | Description |
| --- | --- | --- |
| `SMB_HOST` | *required* | SMB server host, optionally with `:port` (default port 445) |
| `SMB_SHARE` | *required* | Share name, for example `Data` |
| `SMB_USER` | empty | Username; leave empty for a guest or anonymous session |
| `SMB_PASSWORD` | empty | Password |
| `SMB_DOMAIN` | empty | NTLM domain or workgroup |
| `SMB_TIMEOUT` | `30s` | Bounds the connection and every operation |
| `SMB_READ_MAX_BYTES` | `1048576` | Largest file `read` will pull over the wire |
| `SMB_READ_ONLY` | `false` | When true, only `list`, `search` and `read` are exposed |
| `SMB_DISABLE_DELETE` | `false` | When true, `delete` is not exposed, and `write` and `move` refuse to overwrite anything |
| `SMB_TOOL_PREFIX` | `samba_` | Prepended to every tool name; set it to an empty value for unprefixed names |

`SMB_DISABLE_DELETE` protects existing data rather than just hiding one tool: with it set, writing over an existing file and moving onto an existing destination are both refused, because either would destroy the previous content.

**List Input Format** (`samba_list`)**:**
```json
{
  "path": "models/qwen"
}
```

**Search Input Format** (`samba_search`)**:**
```json
{
  "pattern": "*.gguf",
  "path": "models",
  "max_depth": 10,
  "max_results": 100
}
```

Search matches names only, never file contents, so it stays cheap on a share full of large files. It reports `truncated` when it stopped at `max_results`.

**Read Input Format** (`samba_read`)**:**
```json
{
  "path": "models/qwen/config.json",
  "offset": 0,
  "limit": 50
}
```

**Write Input Format** (`samba_write`)**:**
```json
{
  "path": "notes/todo.txt",
  "content": "file content here"
}
```

**Move Input Format** (`samba_move`)**:**
```json
{
  "from": "notes/todo.txt",
  "to": "archive/done.txt",
  "overwrite": false
}
```

**Delete Input Format** (`samba_delete`)**:**
```json
{
  "path": "archive",
  "recursive": true
}
```

**Docker Image:**
```bash
docker run -i --rm \
  -e SMB_HOST=192.168.1.10 \
  -e SMB_SHARE=Data \
  -e SMB_USER=nasuser \
  -e SMB_PASSWORD=secret \
  ghcr.io/mudler/mcps/samba:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "samba": {
          "command": "docker",
          "env": {
            "SMB_HOST": "192.168.1.10",
            "SMB_SHARE": "Data",
            "SMB_USER": "nasuser",
            "SMB_PASSWORD": "secret",
            "SMB_DISABLE_DELETE": "true"
          },
          "args": [
            "run", "-i", "--rm",
            "-e", "SMB_HOST", "-e", "SMB_SHARE",
            "-e", "SMB_USER", "-e", "SMB_PASSWORD",
            "-e", "SMB_DISABLE_DELETE",
            "ghcr.io/mudler/mcps/samba:master"
          ]
        }
      }
    }
```

### 🤖 Claude Server

An MCP server for controlling Claude Code CLI sessions asynchronously. Start sessions, monitor progress, retrieve logs, and manage multiple concurrent Claude Code processes. It follows the same pattern as the opencode MCP server but is specifically designed for Claude Code.

**Features:**
- Background task delegation to Claude Code
- Async session management with unique session IDs
- Start Claude Code sessions with full command-line option support
- Monitor session status (starting, running, completed, failed, stopped)
- Retrieve stdout/stderr logs from sessions
- Stop running sessions gracefully
- List all sessions with filtering by status
- Tool restrictions via `--allowedTools` and `--tools` flags
- Environment passthrough to Claude subprocesses
- Configurable concurrent session limits
- Automatic log cleanup based on retention policy

**Tools:**
- `start_session` - Start a new Claude session with a prompt and options
- `get_session_status` - Get the current status of a session by ID
- `get_session_logs` - Retrieve stdout and stderr logs from a session
- `stop_session` - Stop a running session
- `list_sessions` - List all sessions with optional status filtering

**Configuration:**
- `CLAUDE_SESSION_DIR` - Directory for session state and logs (default: `/tmp/claude-sessions`)
- `CLAUDE_BINARY` - Path to Claude binary (default: `claude`)
- `CLAUDE_MAX_SESSIONS` - Maximum number of concurrent sessions (default: `10`)
- `CLAUDE_LOG_RETENTION_HOURS` - Hours to retain session logs before cleanup (default: `24`)
- `CLAUDE_WORK_DIR` - Working directory for Claude processes (default: `/root`)
- `CLAUDE_MODEL` - Default model to use (e.g., `sonnet`, `opus`)
- `CLAUDE_AGENT` - Specify an agent for sessions
- `CLAUDE_FORMAT` - Output format (default: `json`)
- `CLAUDE_SHARE` - Whether to share sessions (`true`/`false`)
- `CLAUDE_ATTACH` - Files to attach to sessions
- `CLAUDE_PORT` - Port for remote control
- `CLAUDE_VARIANT` - Model variant
- `CLAUDE_ALLOWED_TOOLS` - Tools allowed without prompting (e.g., `"Bash,Read,Edit"`)
- `CLAUDE_TOOLS` - Restrict which tools Claude can use (e.g., `"Bash,Read,Edit"`)
- `CLAUDE_TOOL_START_SESSION_NAME` - Override the start_session tool name
- `CLAUDE_TOOL_GET_SESSION_STATUS_NAME` - Override the get_session_status tool name
- `CLAUDE_TOOL_GET_SESSION_LOGS_NAME` - Override the get_session_logs tool name
- `CLAUDE_TOOL_STOP_SESSION_NAME` - Override the stop_session tool name
- `CLAUDE_TOOL_LIST_SESSIONS_NAME` - Override the list_sessions tool name

**Start Session Example:**
```json
{
  "message": "Find and fix the bug in auth.py",
  "allowed_tools": "Bash,Read,Edit",
  "title": "Bug Fix Task"
}
```

**Start Session Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "starting",
  "message": "Session started successfully"
}
```

**Get Session Status Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "pid": "12345",
  "exit_code": "0",
  "created_at": "2025-01-15T10:30:00Z",
  "started_at": "2025-01-15T10:30:01Z",
  "stopped_at": "2025-01-15T10:30:15Z",
  "duration": "14s"
}
```

**Get Session Logs Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "stdout": "Working on the bug fix...",
  "stderr": "",
  "line_count": 50
}
```

**Adding the Server:**
```bash
claude mcp add claude-mcp -- npx -y mudler/MCPs/claude
```

**Docker Image:**
```bash
docker run -e CLAUDE_MAX_SESSIONS=5 -e CLAUDE_LOG_RETENTION_HOURS=48 ghcr.io/mudler/mcps/claude:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "claude": {
          "command": "docker",
          "env": {
            "CLAUDE_MAX_SESSIONS": "5",
            "CLAUDE_LOG_RETENTION_HOURS": "48"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/claude:master"
          ]
        }
      }
    }
```

### 🚀 Opencode Server

An MCP server for controlling opencode AI sessions asynchronously. Start sessions, monitor progress, retrieve logs, and manage multiple concurrent opencode processes.

**Features:**
- Async session management with unique session IDs
- Start opencode sessions with full command-line option support
- Monitor session status (running, completed, failed, stopped)
- Retrieve stdout/stderr logs from sessions
- Stop running sessions gracefully
- List all sessions with filtering by status
- Configurable concurrent session limits
- Automatic log cleanup based on retention policy
- Ephemeral sessions (do not survive server restarts)

**Tools:**
- `start_session` - Start a new opencode session with a message and options
- `get_session_status` - Get the current status of a session by ID
- `get_session_logs` - Retrieve stdout and stderr logs from a session
- `stop_session` - Stop a running session
- `list_sessions` - List all sessions with optional status filtering

**Configuration:**
- `OPENCODE_SESSION_DIR` - Directory for session state and logs (default: `/tmp/opencode-sessions`)
- `OPENCODE_BINARY` - Path to opencode binary (default: `/usr/local/bin/opencode`)
- `OPENCODE_MAX_SESSIONS` - Maximum number of concurrent sessions (default: `10`)
- `OPENCODE_LOG_RETENTION_HOURS` - Hours to retain session logs before cleanup (default: `24`)
- `OPENCODE_CONFIG` - Path to opencode config file
- `OPENCODE_CONFIG_CONTENT` - Inline config as JSON string
- `OPENCODE_MODEL` - Model to use in provider/model format (e.g., `openai/gpt-4`)
- `OPENCODE_FORMAT` - Output format: `default` (formatted) or `json` (raw JSON events) (default: `json`)
- `OPENCODE_AGENT` - Agent to use for sessions
- `OPENCODE_SHARE` - Share sessions: `true` or `false` (default: `false`)
- `OPENCODE_VARIANT` - Model variant for provider-specific reasoning effort
- `OPENCODE_WORKDIR` - Directory where opencode starts (default: `/root`)

**Start Session Example:**
```json
{
  "message": "Explain quantum computing",
  "title": "Quantum Computing Explanation"
}
```

**Start Session Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "starting",
  "message": "Session started successfully"
}
```

**Get Session Status Example:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Get Session Status Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "pid": "12345",
  "exit_code": "0",
  "created_at": "2025-01-15T10:30:00Z",
  "started_at": "2025-01-15T10:30:01Z",
  "stopped_at": "2025-01-15T10:30:15Z",
  "duration": "14s"
}
```

**Get Session Logs Example:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "lines": 50
}
```

**Get Session Logs Output:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "stdout": "Quantum computing is a form of computing that takes advantage...",
  "stderr": "",
  "line_count": 50
}
```

**Docker Image:**
```bash
docker run -e OPENCODE_MAX_SESSIONS=5 -e OPENCODE_LOG_RETENTION_HOURS=48 ghcr.io/mudler/mcps/opencode:latest
```

**With model configuration:**
```bash
docker run -e OPENCODE_MODEL=openai/gpt-4 -e OPENCODE_FORMAT=json ghcr.io/mudler/mcps/opencode:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "opencode": {
          "command": "docker",
          "env": {
            "OPENCODE_MAX_SESSIONS": "5",
            "OPENCODE_LOG_RETENTION_HOURS": "48"
          },
          "args": [
            "run", "-i", "--rm",
            "ghcr.io/mudler/mcps/opencode:master"
          ]
        }
      }
    }
```

### 🎬 Jellyfin Server

A Jellyfin media server MCP that provides tools for searching, browsing, and controlling your Jellyfin media library.

**Features:**
- Search and browse media (movies, series, episodes) with filtering and pagination
- Get detailed item metadata (cast, media quality, external IDs)
- TV show navigation (seasons, episodes, next up)
- Playback control on active sessions (pause, stop, seek, next/prev)
- User data management (favorites, played status)
- Similar item recommendations
- Configurable tool registration (enable only the tools you need)

**Tools:**
- `search` - Full-text search across all media types
- `browse_library` - Browse/filter items with sorting and pagination
- `get_item` - Get full metadata for a specific item
- `list_libraries` - List all media libraries
- `get_similar` - Find similar items for recommendations
- `get_latest` - Get recently added items (requires user ID)
- `get_seasons` - List seasons for a TV series
- `get_episodes` - List episodes, optionally by season
- `get_next_up` - Get next unwatched episodes (requires user ID)
- `get_sessions` - List active playback sessions
- `playback_control` - Control playback (Pause, Unpause, Stop, NextTrack, PreviousTrack, Seek)
- `set_favorite` - Mark/unmark items as favorites (requires user ID)
- `set_played` - Mark/unmark items as played (requires user ID)

**Configuration:**
- `JELLYFIN_URL` - Jellyfin server URL (required, e.g. `http://jellyfin:8096`)
- `JELLYFIN_API_KEY` - API key for authentication (required, create in Dashboard > Admin > API Keys)
- `JELLYFIN_USERNAME` - Username to resolve for user-scoped operations (optional, easier alternative to user ID)
- `JELLYFIN_USER_ID` - User ID for user-scoped operations like favorites, played, next-up (optional, use JELLYFIN_USERNAME instead)
- `JELLYFIN_TOOLS` - Comma-separated list of tools to register, or "all" (default: all)

**Docker Image:**
```bash
docker run -e JELLYFIN_URL=http://jellyfin:8096 -e JELLYFIN_API_KEY=your-key ghcr.io/mudler/mcps/jellyfin:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "jellyfin": {
          "command": "docker",
          "env": {
            "JELLYFIN_URL": "http://your-jellyfin:8096",
            "JELLYFIN_API_KEY": "your-api-key",
            "JELLYFIN_USERNAME": "your-username"
          },
          "args": [
            "run", "-i", "--rm",
            "-e", "JELLYFIN_URL",
            "-e", "JELLYFIN_API_KEY",
            "-e", "JELLYFIN_USERNAME",
            "ghcr.io/mudler/mcps/jellyfin:master"
          ]
        }
      }
    }
```

### openHAB

Read and control a home through [openHAB](https://www.openhab.org/)'s REST API:
list items and their state, send commands, check whether the devices behind them
are online, and run rules.

| Variable | Default | Description |
| --- | --- | --- |
| `OPENHAB_URL` | required | Base URL, for example `http://openhab:8080` or `https://10.0.0.5:8443` |
| `OPENHAB_API_TOKEN` | empty | openHAB API token; preferred over basic auth |
| `OPENHAB_USERNAME` | empty | Basic-auth user, used when no token is set |
| `OPENHAB_PASSWORD` | empty | Basic-auth password |
| `OPENHAB_TIMEOUT` | `30s` | Bounds every request |
| `OPENHAB_CA_CERT` | empty | Path to a PEM bundle, for an instance behind a private CA |
| `OPENHAB_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification entirely |
| `OPENHAB_READ_ONLY` | `false` | When true, only the read tools are exposed |

Tools: `list_items`, `get_item`, `send_command`, `update_item_state`,
`list_things`, `get_thing_status`, `list_rules`, `run_rule_now`.

`send_command` sends a command, which travels through rules and bindings to the
device. `update_item_state` sets an item's state without triggering rules. They
are different acts and the server keeps them apart.

openHAB's default HTTPS certificate is self-signed and carries no
`subjectAltName`, so no CA bundle can validate it — `OPENHAB_INSECURE_SKIP_VERIFY=true`
is the way to reach such an instance until its certificate is reissued.

```bash
docker run -i --rm \
  -e OPENHAB_URL=https://10.0.0.5:8443 \
  -e OPENHAB_API_TOKEN=oh.mytoken.xxxxx \
  -e OPENHAB_INSECURE_SKIP_VERIFY=true \
  ghcr.io/mudler/mcps/openhab:latest
```

```json
{
    "mcpServers": {
        "openhab": {
            "command": "docker",
            "args": [
                "run", "-i", "--rm",
                "-e", "OPENHAB_URL",
                "-e", "OPENHAB_API_TOKEN",
                "ghcr.io/mudler/mcps/openhab:master"
            ],
            "env": {
                "OPENHAB_URL": "http://your-openhab:8080",
                "OPENHAB_API_TOKEN": "oh.mytoken.xxxxx"
            }
        }
    }
}
```

### 🐙 GitHub Server

A read-only GitHub MCP for reading issues and pull requests — their description, metadata, and discussion comments, plus the unified diff for PRs on request.

**Features:**
- Read issues (title, body, labels, state, comments)
- Read pull requests (title, body, base/head refs, draft/merged flags, comments)
- Optionally fetch the unified diff of a pull request
- Accepts either `owner/repo/number` or a full GitHub URL
- Works anonymously on public repos (rate-limited) or authenticated via `GITHUB_TOKEN`
- Comment truncation to protect context size

**Tools:**
- `get_issue` - Fetch an issue with its body and comments
- `get_pull_request` - Fetch a PR with its body, comments, and (optionally) its unified diff

**Configuration:**
- `GITHUB_TOKEN` - Personal access token (optional; required for private repos and higher rate limits)
- `GITHUB_API_URL` - API base URL (default: `https://api.github.com`; set for GitHub Enterprise)
- `GITHUB_MAX_COMMENT_LENGTH` - Max characters per comment/body before truncation (default: 4000)
- `GITHUB_TOOLS` - Comma-separated list of tools to register, or `all` (default: all)

**Docker Image:**
```bash
docker run -e GITHUB_TOKEN=your-token ghcr.io/mudler/mcps/github:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "github": {
          "command": "docker",
          "env": {
            "GITHUB_TOKEN": "your-token"
          },
          "args": [
            "run", "-i", "--rm",
            "-e", "GITHUB_TOKEN",
            "ghcr.io/mudler/mcps/github:master"
          ]
        }
      }
    }
```

### 🖥️ CUA Server

A computer-use server that gives a model a real XFCE desktop and a real Chrome browser inside a container, viewable live over noVNC. It implements no tools of its own: it starts the `computer` and `browser` MCP servers from [nib](https://github.com/mudler/nib) on in-memory transports and re-exposes their merged tool list on a single stdio server.

Unlike every other server in this repository, this one is **container-only**. The binary alone is not useful: it needs the XFCE desktop, the session D-Bus, and the `cua-driver serve` daemon that the image starts inside the desktop session.

**Features:**
- Desktop control addressed by accessibility element index, not just pixel coordinates (AT-SPI via `cua-driver`)
- Browser control over an accessibility snapshot with `@eN` element refs
- Chrome runs on the same desktop, so the browser and desktop tools compose
- Live view of what the model is doing at `http://localhost:6901`
- Screenshots are returned as image content, so a vision-capable model is required

**Tools:**
- `computer_use` - Desktop control. Actions: `capture`, `click`, `double_click`, `right_click`, `middle_click`, `drag`, `scroll`, `type`, `key`, `set_value`, `wait`, `list_apps`, `open_app`, `close_app`, `focus_app`. Capture modes: `som` (numbered elements, default), `vision`, `ax`
- `browser_navigate` - Open a URL and return a snapshot of the page's interactive elements
- `browser_snapshot` - Re-read the current page's accessibility tree for fresh refs
- `browser_click` - Click the element identified by an `@eN` ref
- `browser_type` - Focus an element, clear it, and type into it
- `browser_press` - Send a single named key (`Enter`, `Tab`, `Escape`, …) to whatever has focus
- `browser_scroll` - Scroll the viewport up or down
- `browser_vision` - Screenshot the current page for visual inspection

**Configuration:**
- `CUA_ENABLE_COMPUTER` - Register `computer_use` (default: `true`)
- `CUA_ENABLE_BROWSER` - Register the `browser_*` tools (default: `true`). Setting both this and `CUA_ENABLE_COMPUTER` to `false` is a fatal error
- `CUA_TOOLS` - Comma-separated list of tools to register, or `all` (default: all)
- `CUA_DRIVER_CMD` - Path to the `cua-driver` binary (default: `cua-driver`)
- `CUA_CHROME_PATH` - Chrome binary override (default: nib auto-discovers `/usr/bin/google-chrome`, then `/usr/bin/chromium`)
- `CUA_BROWSER_PROFILE_DIR` - Chrome profile directory (default: `dante-browser-profile` under the user cache dir, i.e. `/home/cua/.cache/` in this image — it lives and dies with the container unless you mount a volume)
- `CUA_ALLOW_PRIVATE_URLS` - Allow the browser to navigate to localhost and RFC1918 addresses (default: `false`)
- `CUA_READY_TIMEOUT` - Budget for the startup readiness gate: X display wait, driver probe, and each upstream handshake (default: `60s`). Values that do not parse, or that are zero or negative, fall back to the default
- `COGITO_LOG_LEVEL` / `LOG_FORMAT` - nib's log level and format (`json` for JSON). Logs always go to stderr; stdout carries only JSON-RPC

Inherited from the base image, and useful:
- `VNC_RESOLUTION` - Desktop resolution (default: `1024x768`). Raising it raises the token cost of every screenshot proportionally
- `VNC_COL_DEPTH` - Colour depth (default: `24`)
- `VNC_PW` - VNC password. **If unset, the VNC server runs with no authentication at all**
- `VNC_PORT` / `NOVNC_PORT` - Ports for TigerVNC and noVNC (defaults: `5901`, `6901`)

**Docker Image:**
```bash
docker run -i --rm -p 6901:6901 ghcr.io/mudler/mcps/cua:latest
```

Then open `http://localhost:6901` in a browser to watch the desktop live. Port `5901` is also exposed for a native VNC client; publish it only if you need it, and set `VNC_PW` when you do.

The image is built with `make build MCP_SERVER=cua`, which uses `cua/Dockerfile` rather than the shared one.

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "cua": {
          "command": "docker",
          "args": [
            "run", "-i", "--rm", "-p", "6901:6901",
            "ghcr.io/mudler/mcps/cua:master"
          ]
        }
      }
    }
```

**Security posture:**

This container is the security boundary, and it is a soft one. Treat it as disposable and keep it isolated.

- **Chrome runs as root with `--no-sandbox`.** The whole container runs as root because supervisord needs it to drop privileges per program, and Chrome refuses to sandbox itself as root, so a wrapper on `/usr/bin/google-chrome` and `/usr/bin/chromium` passes `--no-sandbox`. A renderer compromise therefore yields root inside the container. This is a deliberate, accepted trade-off — dropping Chrome to an unprivileged user costs the shared X session and the AT-SPI tree that `computer_use` depends on.
- **Do not run this container with `--network host` or host path mounts.** Given the point above, either one turns a browser compromise into a host compromise.
- **The container is a real interactive desktop.** Anything reachable on its network is reachable by whatever the model drives — which is why `CUA_ALLOW_PRIVATE_URLS` defaults to `false` and why placing this container on a network with internal services deserves thought.
- **noVNC and VNC are unauthenticated unless `VNC_PW` is set.** Anyone who can reach the published port has full keyboard and mouse control of the desktop. Bind them to localhost or leave them unpublished on shared hosts.
- nib hard-blocks a small set of key combos (e.g. `cmd+ctrl+q`, `win+l`) and typed-text patterns (`curl … | bash`, `sudo rm -rf`, fork bombs). This is a guardrail against accidents, not a sandbox — there is no interactive approval prompt in this server.

**Notes and limitations:**
- **The image is large: roughly 6.4 GB**, of which about 5.7 GB is the `trycua/cua-xfce` base. Budget disk and pull time accordingly.
- **The base image is `trycua/cua-xfce:latest`, unpinned**, as is Google's Chrome apt repository. A rebuild can therefore pick up a different desktop or a different Chrome than the last one did. The Chrome version actually shipped is recorded in the image at `/etc/cua-chrome-version`, and the build fails loudly if the base's `xstartup.sh` changes shape under the daemon injection.
- **`/dev/uinput` is not available in a container**, so `cua-driver` injects input via `XSendEvent`. Right, middle, and double clicks may not register on some GTK and Qt applications. Clicks addressed by element index go through AT-SPI and are unaffected, which is why element-based addressing is preferred.
- Startup takes tens of seconds: the desktop, then the accessibility bus, then the `cua-driver serve` daemon must all come up before the first tool call. The entrypoint waits up to 120s for the driver socket.
- Only `linux/amd64` is built. `trycua` publishes the base for amd64 only, and Google ships no arm64 Chrome `.deb`; an arm64 build fails at the Chrome install step.

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
make MCP_SERVER=openweathermap build
make MCP_SERVER=wait build
make MCP_SERVER=memory build
make MCP_SERVER=shell build
make MCP_SERVER=ssh build
make MCP_SERVER=scripts build
make MCP_SERVER=localrecall build
make MCP_SERVER=todo build
make MCP_SERVER=mailbox build
make MCP_SERVER=filesystem build
make MCP_SERVER=claude build
make MCP_SERVER=jellyfin build
make MCP_SERVER=github build

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

- `ghcr.io/mudler/mcps/<component>:latest` - Latest component tagged version
- `ghcr.io/mudler/mcps/<component>:v<version>` - Specific tagged versions
- `ghcr.io/mudler/mcps/<component>:master` - Development versions

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

### 🤖 Sub-Agent Server

A Model Context Protocol (MCP) server that allows sending chat completion messages to any OpenAI-compatible endpoint, with support for background job tracking using goroutines and in-memory storage with TTL.

**Features:**
- Send chat completion requests to OpenAI-compatible endpoints
- Background job tracking with asynchronous execution
- In-memory storage with configurable TTL for results
- Three MCP tools for managing sub-agent calls

**Tools:**
- `sub_agent_send` - Send a chat completion message to an OpenAI-compatible endpoint
- `sub_agent_list` - List all active sub-agent calls with their status
- `sub_agent_get_result` - Get the result of a completed sub-agent call by task ID

**Configuration:**
- `OPENAI_BASE_URL` - The base URL for the OpenAI API endpoint (default: `https://api.openai.com/v1`)
- `OPENAI_MODEL` - The model to use for chat completions (default: `gpt-3.5-turbo`)
- `OPENAI_API_KEY` - The API key for authentication (required)
- `TTL` - Time-to-live for stored results in Go duration format (default: `1h`)

**Docker Image:**
```bash
docker run -e OPENAI_API_KEY=your-key ghcr.io/mudler/mcps/sub-agent:latest
```

**LocalAI configuration (to add to the model config):**
```yaml
mcp:
  stdio: |
    {
      "mcpServers": {
        "sub-agent": {
          "command": "docker",
          "env": {
            "OPENAI_BASE_URL": "https://your-openai-compatible-endpoint/v1",
            "OPENAI_MODEL": "your-model",
            "OPENAI_API_KEY": "your-api-key",
            "TTL": "2h"
          },
          "args": [
            "run", "-i", "--rm",
            "-e", "OPENAI_BASE_URL",
            "-e", "OPENAI_MODEL",
            "-e", "OPENAI_API_KEY",
            "-e", "TTL",
            "ghcr.io/mudler/mcps/sub-agent:master"
          ]
        }
      }
    }
```

For more details, see the [sub-agent README](./sub-agent/README.md).

