package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input type for executing shell scripts
type ExecuteCommandInput struct {
	Script  string `json:"script" jsonschema:"the shell script to execute"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"optional timeout in seconds (default: 30)"`
}

// Output type for script execution results
type ExecuteCommandOutput struct {
	Script   string `json:"script" jsonschema:"the script that was executed"`
	Stdout   string `json:"stdout" jsonschema:"standard output from the script"`
	Stderr   string `json:"stderr" jsonschema:"standard error from the script"`
	ExitCode int    `json:"exit_code" jsonschema:"exit code of the script (0 means success)"`
	Success  bool   `json:"success" jsonschema:"whether the script executed successfully"`
	Error    string `json:"error,omitempty" jsonschema:"error message if execution failed"`
}

// getShellCommand returns the shell command to use, defaulting to "sh" if not set
func getShellCommand() string {
	shellCmd := os.Getenv("SHELL_CMD")
	if shellCmd == "" {
		shellCmd = "sh -c"
	}
	return shellCmd
}

// ExecuteCommand executes a shell script and returns the output
func ExecuteCommand(ctx context.Context, req *mcp.CallToolRequest, input ExecuteCommandInput) (
	*mcp.CallToolResult,
	ExecuteCommandOutput,
	error,
) {
	// Set default timeout if not provided
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	// Create a context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Get shell command from environment variable (default: "sh")
	shellCmd := getShellCommand()

	// Parse shell command - support both single command and command with args
	shellParts := strings.Fields(shellCmd)
	shellExec := shellParts[0]
	shellArgs := append(shellParts[1:], input.Script)

	// Execute script using the configured shell
	cmd := exec.CommandContext(cmdCtx, shellExec, shellArgs...)

	// Create buffers to capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute command
	err := cmd.Run()

	exitCode := 0
	success := true
	errorMsg := ""

	if err != nil {
		success = false
		errorMsg = err.Error()

		// Try to get exit code if available
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Context timeout or other error
			if cmdCtx.Err() == context.DeadlineExceeded {
				errorMsg = "Command timed out"
			}
			exitCode = -1
		}
	}

	output := ExecuteCommandOutput{
		Script:   input.Script,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Success:  success,
		Error:    errorMsg,
	}

	return nil, output, nil
}

func main() {
	// Create MCP server for shell command execution
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "shell",
		Version: "v1.0.0",
	}, nil)

	// Add tool for executing shell scripts
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute_command",
		Description: "Execute a shell script and return the output, exit code, and any errors. The shell command can be configured via SHELL_CMD environment variable (default: 'sh')",
	}, ExecuteCommand)

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
