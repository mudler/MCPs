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

### 🧠 Memory Server

A persistent memory storage server that allows AI models to store, retrieve, and manage information across sessions.

**Features:**
- Persistent JSON file storage
- Add, list, and remove memory entries
- Unique ID generation for each entry
- Timestamp tracking for entries
- Configurable storage location
- JSON schema validation for inputs/outputs

**Tools:**
- `add_memory` - Add a new entry to memory storage
- `list_memory` - List all memory entries
- `remove_memory` - Remove a memory entry by ID
- `search_memory` - Search memory entries by content (case-insensitive)

**Configuration:**
- `MEMORY_FILE_PATH` - Environment variable to set the memory file path (default: `/data/memory.json`)

**Memory Entry Format:**
```json
{
  "id": "1703123456789000000",
  "content": "User prefers coffee over tea",
  "created_at": "2023-12-21T10:30:56.789Z"
}
```

**Search Response Format:**
```json
{
  "query": "coffee",
  "results": [
    {
      "id": "1703123456789000000",
      "content": "User prefers coffee over tea",
      "created_at": "2023-12-21T10:30:56.789Z"
    }
  ],
  "count": 1
}
```

**Docker Image:**
```bash
docker run -e MEMORY_FILE_PATH=/custom/path/memory.json ghcr.io/mudler/mcps/memory:latest
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
            "MEMORY_FILE_PATH": "/data/memory.json"
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
make MCP_SERVER=memory build

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
- `ghcr.io/mudler/mcps/memory:latest` - Latest Memory server
- `ghcr.io/mudler/mcps/memory:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/memory:master` - Development versions
- `ghcr.io/mudler/mcps/homeassistant:latest` - Latest Home Assistant server
- `ghcr.io/mudler/mcps/homeassistant:v1.0.0` - Tagged versions
- `ghcr.io/mudler/mcps/homeassistant:master` - Development versions

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
