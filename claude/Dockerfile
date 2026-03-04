# Build stage for MCP server
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o claude-mcp-server ./claude/

# Final stage - use Debian instead of Alpine for glibc compatibility
FROM ghcr.io/anomalyco/opencode

# Install required runtime dependencies
RUN apk add git curl bash

# Install GitHub CLI
RUN echo "@community http://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories && apk add github-cli@community

# Copy the MCP server binary
COPY --from=builder /app/claude-mcp-server /usr/local/bin/claude-mcp-server

# Set environment variables
ENV CLAUDE_SESSION_DIR=/root
ENV CLAUDE_BINARY=/usr/local/bin/claude
ENV CLAUDE_MAX_SESSIONS=10
ENV CLAUDE_LOG_RETENTION_HOURS=24
ENV CLAUDE_FORMAT=json
ENV CLAUDE_SHARE=false

# Optional environment variables (uncomment to use):
# ENV CLAUDE_MODEL=openai/gpt-4
# ENV CLAUDE_AGENT=
# ENV CLAUDE_VARIANT=
# ENV CLAUDE_ATTACH=
# ENV CLAUDE_PORT=0
# ENV CLAUDE_ALLOWED_TOOLS=
# ENV CLAUDE_TOOLS=
# ENV CLAUDE_CREDENTIALS=  # JSON content of ~/.claude/.credentials.json for OAuth auth
# ENV ANTHROPIC_API_KEY=   # Anthropic API key (alternative to OAuth credentials)

# Run the MCP server
ENTRYPOINT ["claude-mcp-server"]
