package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExecutorConfig represents a single script/program executor configuration
type ExecutorConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command,omitempty"`
	Path        string            `json:"path,omitempty"`
	Content     string            `json:"content,omitempty"`
	Interpreter string            `json:"interpreter,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// Input struct for script/program execution
type ExecuteInput struct {
	Args []string `json:"args,omitempty" jsonschema:"arguments to pass to the script or program"`
}

// Output struct for execution results
type ExecuteOutput struct {
	Stdout     string `json:"stdout" jsonschema:"standard output from execution"`
	Stderr     string `json:"stderr" jsonschema:"standard error from execution"`
	ExitCode   int    `json:"exit_code" jsonschema:"exit code from execution"`
	DurationMs int    `json:"duration_ms" jsonschema:"execution duration in milliseconds"`
}

// detectInterpreter attempts to detect the interpreter from shebang or file extension
func detectInterpreter(content string, path string) string {
	// Check for shebang in content
	if strings.HasPrefix(content, "#!") {
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			shebang := strings.TrimSpace(lines[0][2:]) // Remove "#!"
			parts := strings.Fields(shebang)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}

	// Check file extension if path is provided
	if path != "" {
		ext := filepath.Ext(path)
		switch ext {
		case ".sh", ".bash":
			return "bash"
		case ".py":
			return "python3"
		case ".js":
			return "node"
		case ".rb":
			return "ruby"
		case ".pl":
			return "perl"
		}
	}

	return ""
}

// executeScript runs a script or program with the given configuration
func executeScript(ctx context.Context, config ExecutorConfig, args []string) (ExecuteOutput, error) {
	startTime := time.Now()

	// Determine timeout
	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	var tempFile string
	var err error

	// Handle inline content
	if config.Content != "" {
		// Create temporary file
		var tempFileHandle *os.File
		tempFileHandle, err = os.CreateTemp("", "mcp-script-*")
		if err != nil {
			return ExecuteOutput{}, fmt.Errorf("failed to create temporary file: %w", err)
		}
		tempFile = tempFileHandle.Name()
		defer os.Remove(tempFile)

		// Write content to temp file
		if _, err = tempFileHandle.WriteString(config.Content); err != nil {
			tempFileHandle.Close()
			return ExecuteOutput{}, fmt.Errorf("failed to write script content: %w", err)
		}
		tempFileHandle.Close()

		// Make executable
		if err = os.Chmod(tempFile, 0755); err != nil {
			return ExecuteOutput{}, fmt.Errorf("failed to make script executable: %w", err)
		}

		// Determine interpreter
		interpreter := config.Interpreter
		if interpreter == "" {
			interpreter = detectInterpreter(config.Content, tempFile)
		}

		if interpreter != "" {
			// Execute with interpreter
			cmd = exec.CommandContext(execCtx, interpreter, tempFile)
		} else {
			// Try to execute directly
			cmd = exec.CommandContext(execCtx, tempFile)
		}
	} else if config.Path != "" {
		// Handle file path
		interpreter := config.Interpreter
		if interpreter == "" {
			// Read file to detect shebang
			content, readErr := os.ReadFile(config.Path)
			if readErr == nil {
				interpreter = detectInterpreter(string(content), config.Path)
			}
		}

		if interpreter != "" {
			cmd = exec.CommandContext(execCtx, interpreter, config.Path)
		} else {
			// Execute directly
			cmd = exec.CommandContext(execCtx, config.Path)
		}
	} else if config.Command != "" {
		// Handle direct command/program
		cmdParts := strings.Fields(config.Command)
		if len(cmdParts) == 0 {
			return ExecuteOutput{}, fmt.Errorf("invalid command: %s", config.Command)
		}
		if len(cmdParts) == 1 {
			cmd = exec.CommandContext(execCtx, cmdParts[0])
		} else {
			cmd = exec.CommandContext(execCtx, cmdParts[0], cmdParts[1:]...)
		}
	} else {
		return ExecuteOutput{}, fmt.Errorf("must specify either content, path, or command")
	}

	// Add arguments
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}

	// Set working directory
	if config.WorkingDir != "" {
		cmd.Dir = config.WorkingDir
	}

	// Set environment variables
	if len(config.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Execute command
	err = cmd.Run()
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Context timeout or other error
			if execCtx.Err() == context.DeadlineExceeded {
				return ExecuteOutput{
					Stdout:     stdoutBuf.String(),
					Stderr:     stderrBuf.String() + "\nError: execution timeout",
					ExitCode:   -1,
					DurationMs: int(duration.Milliseconds()),
				}, nil
			}
			return ExecuteOutput{
				Stdout:     stdoutBuf.String(),
				Stderr:     stderrBuf.String() + "\nError: " + err.Error(),
				ExitCode:   -1,
				DurationMs: int(duration.Milliseconds()),
			}, nil
		}
	}

	return ExecuteOutput{
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		ExitCode:   exitCode,
		DurationMs: int(duration.Milliseconds()),
	}, nil
}

// createExecutorHandler creates a handler function for a specific executor configuration
func createExecutorHandler(config ExecutorConfig) func(context.Context, *mcp.CallToolRequest, ExecuteInput) (*mcp.CallToolResult, ExecuteOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ExecuteInput) (*mcp.CallToolResult, ExecuteOutput, error) {
		output, err := executeScript(ctx, config, input.Args)
		if err != nil {
			return nil, ExecuteOutput{}, err
		}
		return nil, output, nil
	}
}

func main() {
	// Read SCRIPTS environment variable
	scriptsJSON := os.Getenv("SCRIPTS")
	if scriptsJSON == "" {
		log.Fatal("SCRIPTS environment variable is required")
	}

	// Parse JSON configuration
	var executors []ExecutorConfig
	if err := json.Unmarshal([]byte(scriptsJSON), &executors); err != nil {
		log.Fatalf("Failed to parse SCRIPTS JSON: %v", err)
	}

	if len(executors) == 0 {
		log.Fatal("SCRIPTS must contain at least one executor configuration")
	}

	// Validate configurations
	for i, executor := range executors {
		if executor.Name == "" {
			log.Fatalf("Executor at index %d: name is required", i)
		}
		if executor.Description == "" {
			log.Fatalf("Executor at index %d: description is required", i)
		}

		// Validate that exactly one of content, path, or command is specified
		hasContent := executor.Content != ""
		hasPath := executor.Path != ""
		hasCommand := executor.Command != ""

		count := 0
		if hasContent {
			count++
		}
		if hasPath {
			count++
		}
		if hasCommand {
			count++
		}

		if count != 1 {
			log.Fatalf("Executor '%s': must specify exactly one of 'content', 'path', or 'command'", executor.Name)
		}
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{Name: "scripts", Version: "v1.0.0"}, nil)

	// Register each executor as a tool
	for _, executor := range executors {
		handler := createExecutorHandler(executor)
		mcp.AddTool(server, &mcp.Tool{
			Name:        executor.Name,
			Description: executor.Description,
		}, handler)
	}

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

