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

FROM ubuntu
# Install required runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    bash \
    wget \
    git jq \
    openssh-client gnupg2 \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-key C99B11DEB97541F0 \
    && apt-add-repository https://cli.github.com/packages \
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
