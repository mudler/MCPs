# Build stage for MCP shell server
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY shell/ ./shell/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o shell-mcp-server ./shell/

# Final stage - use Debian instead of Alpine for glibc compatibility
FROM debian:bookworm-slim

# Install required runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash \
    wget \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN (type -p wget >/dev/null || (apt-get update && apt-get install wget -y)) \
    && mkdir -p -m 755 /etc/apt/keyrings \
    && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    && cat $out | tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && mkdir -p -m 755 /etc/apt/sources.list.d \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update \
    && apt-get install gh -y \
    && rm -rf /var/lib/apt/lists/*

# Copy the MCP server binary
COPY --from=builder /app/shell-mcp-server /usr/local/bin/shell-mcp-server

# Create session directory
RUN mkdir -p /tmp/shell-sessions

# Set environment variables
ENV SHELL_CMD="bash -c"
ENV SHELL_WORKING_DIR="/root"
ENV TOOL_NAME=execute_command

# Run the MCP server
ENTRYPOINT ["shell-mcp-server"]
