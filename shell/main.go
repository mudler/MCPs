package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

// Interactive flags that commonly cause commands to hang
var interactiveFlags = []string{
	"-i", "--interactive", "--tty", "-t",
	"vim", "vi", "nano", "emacs", "less", "more", "top", "htop",
	"ftp", "sftp", "ssh", "ping", "tail -f", "tail -F",
}

// isInteractiveCommand checks if the script contains interactive commands
func isInteractiveCommand(script string) bool {
	scriptLower := strings.ToLower(script)
	for _, flag := range interactiveFlags {
		if strings.Contains(scriptLower, flag) {
			return true
		}
	}
	return false
}

// getShellCommand returns the shell command to use, defaulting to "sh" if not set
func getShellCommand() string {
	shellCmd := os.Getenv("SHELL_CMD")
	if shellCmd == "" {
		shellCmd = "sh -c"
	}
	return shellCmd
}

// getWorkingDirectory returns the working directory from SHELL_WORKING_DIR env var,
// or empty string (use current directory) if not set
func getWorkingDirectory() string {
	return os.Getenv("SHELL_WORKING_DIR")
}

// getTimeout returns the default timeout from SHELL_TIMEOUT env var,
// or 30 seconds if not set or invalid
func getTimeout() int {
	timeoutStr := os.Getenv("SHELL_TIMEOUT")
	if timeoutStr == "" {
		return 30
	}
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout <= 0 {
		return 30
	}
	return timeout
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
		timeout = getTimeout()
	}

	// Warn about interactive commands (but still attempt execution with proper safeguards)
	warningMsg := ""
	if isInteractiveCommand(input.Script) {
		warningMsg = "Warning: Command appears to be interactive and may hang. "
	}

	// Create a context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Get shell command from environment variable (default: "sh -c")
	shellCmd := getShellCommand()

	// Parse shell command - support both single command and command with args
	shellParts := strings.Fields(shellCmd)

	shellExec := shellParts[0]
	shellArgs := []string{}

	if len(shellParts) > 1 {
		shellArgs = append(shellParts[1:], input.Script)
	} else {
		shellArgs = []string{"-c", input.Script}
	}

	// Execute script using the configured shell
	cmd := exec.CommandContext(cmdCtx, shellExec, shellArgs...)

	// Set working directory from environment variable if specified
	if workDir := getWorkingDirectory(); workDir != "" {
		cmd.Dir = workDir
	}

	// CRITICAL FIX: Set process group to enable killing entire process tree
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	// Create buffers to capture stdout and stderr separately
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Set environment variables to force non-interactive mode
	env := os.Environ()
	env = append(env, "CI=true")           // Common flag for CI/CD to disable interactive mode
	env = append(env, "TERM=dumb")          // Force dumb terminal
	env = append(env, "INPUT=/dev/null")    // Redirect stdin from /dev/null
	env = append(env, "NONINTERACTIVE=1")   // Common flag for non-interactive mode
	cmd.Env = env

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
				errorMsg = "Command timed out after " + strconv.Itoa(timeout) + " seconds"
				
				// CRITICAL: Kill the entire process group on timeout
				if cmd.Process != nil && cmd.Process.Pid > 0 {
					// Kill the process group (negative PID)
					syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				}
			}
			exitCode = -1
		}
	}

	// Add warning to stderr if interactive command detected
	if warningMsg != "" {
		stderrBuf.WriteString("\n" + warningMsg)
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
	// Run initialization script if SHELL_INIT_SCRIPT is set
	if initScript := os.Getenv("SHELL_INIT_SCRIPT"); initScript != "" {
		cmd := exec.CommandContext(context.Background(), "sh", "-c", initScript)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Initialization script failed: %v\nOutput: %s", err, string(output))
		}
	}

	// Create MCP server for shell command execution
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "shell",
		Version: "v1.0.0",
	}, nil)

	configurableName := os.Getenv("TOOL_NAME")
	if configurableName == "" {
		configurableName = "execute_command"
	}

	// Add tool for executing shell scripts
	mcp.AddTool(server, &mcp.Tool{
		Name:        configurableName,
		Description: "Execute a shell script and return the output, exit code, and any errors. The shell command can be configured via SHELL_CMD environment variable (default: 'sh -c'). The working directory can be set via SHELL_WORKING_DIR environment variable. The default timeout can be configured via SHELL_TIMEOUT environment variable (default: 30 seconds). An initialization script can be run before server startup via SHELL_INIT_SCRIPT environment variable. NOTE: Interactive commands are detected and force non-interactive mode with CI=true, TERM=dumb, and stdin redirected from /dev/null. Process group management ensures proper cleanup on timeout.",
	}, ExecuteCommand)

	// Run the server
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
