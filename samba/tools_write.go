package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	filePerm = os.FileMode(0o644)
	dirPerm  = os.FileMode(0o755)
)

// errReadOnly and errNoDelete carry the name of the switch that caused a
// refusal, so an operator reading a failed tool call knows which environment
// variable to change.
func errReadOnly(action string) string {
	return fmt.Sprintf("cannot %s: the server is read-only (unset SMB_READ_ONLY to allow it)", action)
}

func errNoDelete(action string) string {
	return fmt.Sprintf("cannot %s: it would destroy existing data and deletion is disabled (unset SMB_DISABLE_DELETE to allow it)", action)
}

type writeInput struct {
	Path    string `json:"path" jsonschema:"file to write, relative to the root of the share"`
	Content string `json:"content" jsonschema:"the content to write"`
}

type writeOutput struct {
	Path         string `json:"path" jsonschema:"the file that was written"`
	BytesWritten int    `json:"bytes_written" jsonschema:"number of bytes written"`
	Success      bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error        string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// write creates or replaces a file, creating parent directories as needed.
func (s *server) write(ctx context.Context, _ *mcp.CallToolRequest, input writeInput) (
	*mcp.CallToolResult,
	writeOutput,
	error,
) {
	if s.cfg.ReadOnly {
		return nil, writeOutput{Error: errReadOnly("write")}, nil
	}

	file, err := cleanPath(input.Path)
	if err != nil {
		return nil, writeOutput{Error: err.Error()}, nil
	}
	if file == "" {
		return nil, writeOutput{Error: "path is the share root, which is a directory"}, nil
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, writeOutput{Path: file, Error: err.Error()}, nil
	}

	switch info, statErr := share.Stat(file); {
	case statErr == nil && info.IsDir():
		return nil, writeOutput{Path: file, Error: fmt.Sprintf("%q is a directory", file)}, nil
	case statErr == nil && s.cfg.DisableDelete:
		// Overwriting destroys the previous content, which is exactly what
		// SMB_DISABLE_DELETE is meant to prevent.
		return nil, writeOutput{Path: file, Error: errNoDelete(fmt.Sprintf("overwrite %q", file))}, nil
	}

	if parent := parentPath(file); parent != "" {
		if err := share.MkdirAll(parent, dirPerm); err != nil {
			return nil, writeOutput{Path: file, Error: err.Error()}, nil
		}
	}

	data := []byte(input.Content)
	if err := share.WriteFile(file, data, filePerm); err != nil {
		return nil, writeOutput{Path: file, Error: err.Error()}, nil
	}

	return nil, writeOutput{
		Path:         file,
		BytesWritten: len(data),
		Success:      true,
	}, nil
}

type moveInput struct {
	From      string `json:"from" jsonschema:"path to move, relative to the root of the share"`
	To        string `json:"to" jsonschema:"destination path, relative to the root of the share"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace the destination if it already exists (default false)"`
}

type moveOutput struct {
	From    string `json:"from" jsonschema:"the path that was moved"`
	To      string `json:"to" jsonschema:"where it was moved to"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// move renames a file or directory within the share.
func (s *server) move(ctx context.Context, _ *mcp.CallToolRequest, input moveInput) (
	*mcp.CallToolResult,
	moveOutput,
	error,
) {
	if s.cfg.ReadOnly {
		return nil, moveOutput{Error: errReadOnly("move")}, nil
	}

	from, err := cleanPath(input.From)
	if err != nil {
		return nil, moveOutput{Error: err.Error()}, nil
	}
	to, err := cleanPath(input.To)
	if err != nil {
		return nil, moveOutput{From: from, Error: err.Error()}, nil
	}
	if from == "" || to == "" {
		return nil, moveOutput{From: from, To: to, Error: "from and to must both name something inside the share, not the share root"}, nil
	}
	if from == to {
		return nil, moveOutput{From: from, To: to, Error: "from and to are the same path"}, nil
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, moveOutput{From: from, To: to, Error: err.Error()}, nil
	}

	if _, err := share.Stat(from); err != nil {
		return nil, moveOutput{From: from, To: to, Error: err.Error()}, nil
	}

	destinationExists := false
	if _, err := share.Stat(to); err == nil {
		destinationExists = true
	}

	switch {
	case destinationExists && !input.Overwrite:
		return nil, moveOutput{
			From:  from,
			To:    to,
			Error: fmt.Sprintf("%q already exists, pass overwrite true to replace it", to),
		}, nil
	case destinationExists && s.cfg.DisableDelete:
		return nil, moveOutput{
			From:  from,
			To:    to,
			Error: errNoDelete(fmt.Sprintf("overwrite %q", to)),
		}, nil
	}

	if parent := parentPath(to); parent != "" {
		if err := share.MkdirAll(parent, dirPerm); err != nil {
			return nil, moveOutput{From: from, To: to, Error: err.Error()}, nil
		}
	}

	// SMB refuses a rename onto an existing name, so clear the way first now
	// that the caller has asked for it and deletion is permitted.
	if destinationExists {
		if err := share.RemoveAll(to); err != nil {
			return nil, moveOutput{From: from, To: to, Error: err.Error()}, nil
		}
	}

	if err := share.Rename(from, to); err != nil {
		return nil, moveOutput{From: from, To: to, Error: err.Error()}, nil
	}

	return nil, moveOutput{From: from, To: to, Success: true}, nil
}

type deleteInput struct {
	Path      string `json:"path" jsonschema:"path to delete, relative to the root of the share"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"delete a directory and everything inside it (default false)"`
}

type deleteOutput struct {
	Path    string `json:"path" jsonschema:"the path that was deleted"`
	Success bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// remove backs the delete tool. It refuses a non-empty directory unless the
// caller explicitly asks to recurse, so a mistyped path cannot take a subtree
// with it.
func (s *server) remove(ctx context.Context, _ *mcp.CallToolRequest, input deleteInput) (
	*mcp.CallToolResult,
	deleteOutput,
	error,
) {
	if s.cfg.ReadOnly {
		return nil, deleteOutput{Error: errReadOnly("delete")}, nil
	}
	if s.cfg.DisableDelete {
		return nil, deleteOutput{Error: errNoDelete("delete")}, nil
	}

	target, err := cleanPath(input.Path)
	if err != nil {
		return nil, deleteOutput{Error: err.Error()}, nil
	}
	if target == "" {
		return nil, deleteOutput{Error: "refusing to delete the share root"}, nil
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, deleteOutput{Path: target, Error: err.Error()}, nil
	}

	info, err := share.Stat(target)
	if err != nil {
		return nil, deleteOutput{Path: target, Error: err.Error()}, nil
	}

	if info.IsDir() && !input.Recursive {
		children, err := share.ReadDir(target)
		if err != nil {
			return nil, deleteOutput{Path: target, Error: err.Error()}, nil
		}
		if len(children) > 0 {
			return nil, deleteOutput{
				Path:  target,
				Error: fmt.Sprintf("%q is not empty, pass recursive true to delete it and its %d entries", target, len(children)),
			}, nil
		}
	}

	remove := share.Remove
	if input.Recursive {
		remove = share.RemoveAll
	}
	if err := remove(target); err != nil {
		return nil, deleteOutput{Path: target, Error: err.Error()}, nil
	}

	return nil, deleteOutput{Path: target, Success: true}, nil
}
