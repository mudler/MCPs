# MCPS - Model Context Protocol Servers

This repository contains Model Context Protocol (MCP) servers that provide various tools and capabilities for AI models.

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
            "run", "-i", "--rm",
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

**LocalAI configuration ( to add to the model config): **
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
