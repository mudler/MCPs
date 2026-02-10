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
FROM debian:bookworm-slim

# Install required runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash \
    wget \
    git \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://opencode.ai/install | bash

RUN ln -s /root/.opencode/bin/opencode /usr/bin/opencode

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
COPY --from=builder /app/opencode-mcp-server /usr/local/bin/opencode-mcp-server

# Verify opencode binary is properly installed
RUN which opencode && opencode --version || echo "Warning: opencode may not be properly installed"

# Create session directory
RUN mkdir -p /tmp/opencode-sessions

# Set environment variables
ENV OPENCODE_SESSION_DIR=/tmp/opencode-sessions
ENV OPENCODE_BINARY=/usr/bin/opencode
ENV OPENCODE_MAX_SESSIONS=10
ENV OPENCODE_LOG_RETENTION_HOURS=24
ENV OPENCODE_FORMAT=json
ENV OPENCODE_SHARE=false

# Optional environment variables (uncomment to use):
# ENV OPENCODE_MODEL=openai/gpt-4
# ENV OPENCODE_AGENT=
# ENV OPENCODE_VARIANT=
# ENV OPENCODE_ATTACH=
# ENV OPENCODE_PORT=0

# Run the MCP server
ENTRYPOINT ["opencode-mcp-server"]
