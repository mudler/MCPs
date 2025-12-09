package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"
)

// Input type for executing scripts on SSH hosts
type ExecuteScriptInput struct {
	Host     string `json:"host" jsonschema:"the SSH host to connect to (required if not set via SSH_HOST env var)"`
	Port     int    `json:"port,omitempty" jsonschema:"the SSH port (default: 22, or SSH_PORT env var)"`
	User     string `json:"user,omitempty" jsonschema:"the SSH username (default: SSH_USER env var)"`
	Password string `json:"password,omitempty" jsonschema:"the SSH password (default: SSH_PASSWORD env var, or use SSH_KEY_PATH)"`
	KeyPath  string `json:"key_path,omitempty" jsonschema:"path to SSH private key file (default: SSH_KEY_PATH env var)"`
	Script   string `json:"script" jsonschema:"the shell script to execute on the remote host"`
	Timeout  int    `json:"timeout,omitempty" jsonschema:"optional timeout in seconds (default: 30)"`
}

// Output type for script execution results
type ExecuteScriptOutput struct {
	Host     string `json:"host" jsonschema:"the SSH host that was connected to"`
	Script   string `json:"script" jsonschema:"the script that was executed"`
	Stdout   string `json:"stdout" jsonschema:"standard output from the script"`
	Stderr   string `json:"stderr" jsonschema:"standard error from the script"`
	ExitCode int    `json:"exit_code" jsonschema:"exit code of the script (0 means success)"`
	Success  bool   `json:"success" jsonschema:"whether the script executed successfully"`
	Error    string `json:"error,omitempty" jsonschema:"error message if execution failed"`
}

// getSSHConfig returns SSH configuration from environment variables or input
func getSSHConfig(input ExecuteScriptInput) (host string, port int, user string, password string, keyPath string, err error) {
	// Host
	host = input.Host
	if host == "" {
		host = os.Getenv("SSH_HOST")
	}
	if host == "" {
		return "", 0, "", "", "", fmt.Errorf("SSH host is required (provide via 'host' parameter or SSH_HOST env var)")
	}

	// Port
	port = input.Port
	if port == 0 {
		portStr := os.Getenv("SSH_PORT")
		if portStr != "" {
			port, err = strconv.Atoi(portStr)
			if err != nil {
				return "", 0, "", "", "", fmt.Errorf("invalid SSH_PORT: %w", err)
			}
		} else {
			port = 22
		}
	}

	// User
	user = input.User
	if user == "" {
		user = os.Getenv("SSH_USER")
	}
	if user == "" {
		return "", 0, "", "", "", fmt.Errorf("SSH user is required (provide via 'user' parameter or SSH_USER env var)")
	}

	// Password
	password = input.Password
	if password == "" {
		password = os.Getenv("SSH_PASSWORD")
	}

	// Key path
	keyPath = input.KeyPath
	if keyPath == "" {
		keyPath = os.Getenv("SSH_KEY_PATH")
	}

	// Must have either password or key
	if password == "" && keyPath == "" {
		return "", 0, "", "", "", fmt.Errorf("SSH authentication required (provide password via 'password' parameter or SSH_PASSWORD env var, or key via 'key_path' parameter or SSH_KEY_PATH env var)")
	}

	return host, port, user, password, keyPath, nil
}

// getShellCommand returns the shell command to use on remote host, defaulting to "sh -c" if not set
func getShellCommand() string {
	shellCmd := os.Getenv("SSH_SHELL_CMD")
	if shellCmd == "" {
		shellCmd = "sh -c"
	}
	return shellCmd
}

// createSSHClient creates an SSH client connection
func createSSHClient(host string, port int, user string, password string, keyPath string) (*ssh.Client, error) {
	// Configure authentication
	var authMethods []ssh.AuthMethod

	// Try key-based authentication first
	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key file: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// Try with passphrase
			passphrase := os.Getenv("SSH_KEY_PASSPHRASE")
			if passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("failed to parse SSH key: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to parse SSH key: %w", err)
			}
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Add password authentication if provided
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, use ssh.FixedHostKey or known_hosts
		Timeout:         10 * time.Second,
	}

	// Connect to SSH server
	address := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH host: %w", err)
	}

	return client, nil
}

// ExecuteScript executes a shell script on a remote SSH host and returns the output
func ExecuteScript(ctx context.Context, req *mcp.CallToolRequest, input ExecuteScriptInput) (
	*mcp.CallToolResult,
	ExecuteScriptOutput,
	error,
) {
	// Get SSH configuration
	host, port, user, password, keyPath, err := getSSHConfig(input)
	if err != nil {
		return nil, ExecuteScriptOutput{Error: err.Error()}, nil
	}

	// Set default timeout if not provided
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	// Create a context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Create SSH client
	client, err := createSSHClient(host, port, user, password, keyPath)
	if err != nil {
		return nil, ExecuteScriptOutput{
			Host:   host,
			Script: input.Script,
			Error:  err.Error(),
		}, nil
	}
	defer client.Close()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		return nil, ExecuteScriptOutput{
			Host:   host,
			Script: input.Script,
			Error:  fmt.Sprintf("failed to create SSH session: %v", err),
		}, nil
	}
	defer session.Close()

	// Get shell command from environment variable
	shellCmd := getShellCommand()
	shellParts := strings.Fields(shellCmd)

	// Construct the command to execute
	var cmd string
	if len(shellParts) > 1 {
		// Shell command with arguments (e.g., "sh -c")
		// We need to properly quote the script
		cmd = fmt.Sprintf("%s %q", strings.Join(shellParts, " "), input.Script)
	} else {
		// Just shell executable, add -c flag
		shellExec := shellParts[0]
		cmd = fmt.Sprintf("%s -c %q", shellExec, input.Script)
	}

	// Set up stdout and stderr capture
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	// Execute command in a goroutine to support context cancellation
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(cmd)
	}()

	// Wait for completion or timeout
	select {
	case err := <-errChan:
		// Command completed
		exitCode := 0
		success := true
		errorMsg := ""

		if err != nil {
			success = false
			errorMsg = err.Error()

			// Try to get exit code from SSH session
			if exitError, ok := err.(*ssh.ExitError); ok {
				exitCode = exitError.ExitStatus()
			} else {
				exitCode = -1
			}
		}

		output := ExecuteScriptOutput{
			Host:     host,
			Script:   input.Script,
			Stdout:   stdoutBuf.String(),
			Stderr:   stderrBuf.String(),
			ExitCode: exitCode,
			Success:  success,
			Error:    errorMsg,
		}

		return nil, output, nil
	case <-cmdCtx.Done():
		// Timeout or cancellation
		session.Close()
		client.Close()
		return nil, ExecuteScriptOutput{
			Host:   host,
			Script: input.Script,
			Error:  "Command timed out",
		}, nil
	}
}

func main() {
	// Create MCP server for SSH script execution
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ssh",
		Version: "v1.0.0",
	}, nil)

	configurableName := os.Getenv("TOOL_NAME")
	if configurableName == "" {
		configurableName = "remote_ssh"
	}

	// Add tool for executing scripts on SSH hosts
	mcp.AddTool(server, &mcp.Tool{
		Name:        configurableName,
		Description: "Execute a shell script on a remote SSH host and return the output, exit code, and any errors. SSH connection details can be provided via parameters or environment variables (SSH_HOST, SSH_PORT, SSH_USER, SSH_PASSWORD, SSH_KEY_PATH). The remote shell command can be configured via SSH_SHELL_CMD environment variable (default: 'sh -c')",
	}, ExecuteScript)

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
