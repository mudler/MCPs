# Build stage for MCP server
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY opencode/ ./opencode/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o opencode-mcp-server ./opencode/

# Final stage - use Debian instead of Alpine for glibc compatibility
FROM ghcr.io/anomalyco/opencode

# Install required runtime dependencies
RUN apk add git curl bash

# Install GitHub CLI
RUN echo "@community http://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories && apk add github-cli@community

# Copy the MCP server binary
COPY --from=builder /app/opencode-mcp-server /usr/local/bin/opencode-mcp-server

# Set environment variables
ENV OPENCODE_SESSION_DIR=/root
ENV OPENCODE_BINARY=/usr/local/bin/opencode
ENV OPENCODE_MAX_SESSIONS=10
ENV OPENCODE_SHARE=false

# Persistence and retention settings
ENV OPENCODE_PERSISTENCE_ENABLED=true
ENV OPENCODE_AUTO_SAVE_INTERVAL=30s
ENV OPENCODE_DEFAULT_RETENTION_HOURS=168
ENV OPENCODE_COMPLETED_RETENTION_HOURS=168
ENV OPENCODE_FAILED_RETENTION_HOURS=24
ENV OPENCODE_STOPPED_RETENTION_HOURS=48

# Logging
ENV OPENCODE_LOG_LEVEL=INFO

# Optional environment variables (uncomment to use):
# ENV OPENCODE_MODEL=openai/gpt-4
# ENV OPENCODE_AGENT=
# ENV OPENCODE_VARIANT=
# ENV OPENCODE_ATTACH=
# ENV OPENCODE_PORT=0
# ENV OPENCODE_LOG_FILE=/var/log/opencode/opencode-mcp.log

# Run the MCP server
ENTRYPOINT ["opencode-mcp-server"]
