package main

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMaxDepth   = 10
	defaultMaxResults = 100
)

type listInput struct {
	Path string `json:"path,omitempty" jsonschema:"directory relative to the root of the share, empty for the share root"`
}

type listOutput struct {
	Path    string      `json:"path" jsonschema:"the directory that was listed"`
	Entries []FileEntry `json:"entries" jsonschema:"the files and directories directly inside it"`
	Count   int         `json:"count" jsonschema:"number of entries returned"`
	Success bool        `json:"success" jsonschema:"whether the operation succeeded"`
	Error   string      `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// list returns the immediate children of a directory. It does not recurse:
// use search for that.
func (s *server) list(ctx context.Context, _ *mcp.CallToolRequest, input listInput) (
	*mcp.CallToolResult,
	listOutput,
	error,
) {
	dir, err := cleanPath(input.Path)
	if err != nil {
		return nil, listOutput{Error: err.Error()}, nil
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, listOutput{Path: dir, Error: err.Error()}, nil
	}

	infos, err := share.ReadDir(dir)
	if err != nil {
		return nil, listOutput{Path: dir, Error: err.Error()}, nil
	}

	entries := make([]FileEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, newFileEntry(dir, info))
	}
	sortEntries(entries)

	return nil, listOutput{
		Path:    dir,
		Entries: entries,
		Count:   len(entries),
		Success: true,
	}, nil
}

// sortEntries puts directories first, then orders case-insensitively by name,
// which is how a person reading a share expects to see it.
func sortEntries(entries []FileEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

type searchInput struct {
	Pattern    string `json:"pattern" jsonschema:"name to look for: a glob such as *.gguf, or plain text matched as a substring"`
	Path       string `json:"path,omitempty" jsonschema:"directory to search below, empty for the whole share"`
	MaxDepth   int    `json:"max_depth,omitempty" jsonschema:"how many directory levels to descend (default 10)"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"stop after this many matches (default 100)"`
}

type searchOutput struct {
	Matches   []FileEntry `json:"matches" jsonschema:"entries whose name matched the pattern"`
	Count     int         `json:"count" jsonschema:"number of matches returned"`
	Truncated bool        `json:"truncated" jsonschema:"true if the search stopped at max_results and more matches may exist"`
	Success   bool        `json:"success" jsonschema:"whether the operation succeeded"`
	Error     string      `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// search walks the share matching entry names. Only directory listings cross
// the network: file contents are never read, so this stays cheap on a share
// full of large files.
func (s *server) search(ctx context.Context, _ *mcp.CallToolRequest, input searchInput) (
	*mcp.CallToolResult,
	searchOutput,
	error,
) {
	root, err := cleanPath(input.Path)
	if err != nil {
		return nil, searchOutput{Error: err.Error()}, nil
	}

	// Validate the pattern once, up front, rather than discovering it is
	// malformed part way through a walk.
	if _, err := matchName(input.Pattern, "probe"); err != nil {
		return nil, searchOutput{Error: err.Error()}, nil
	}

	maxDepth := input.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, searchOutput{Error: err.Error()}, nil
	}

	matches := []FileEntry{}
	truncated := false

	// Breadth-first, so shallow matches are reported before deep ones when
	// the result cap bites.
	queue := []struct {
		path  string
		depth int
	}{{path: root, depth: 0}}

	for len(queue) > 0 && !truncated {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxDepth {
			continue
		}

		infos, err := share.ReadDir(current.path)
		if err != nil {
			// A directory that disappeared or that we cannot open should not
			// abort the whole search; the root failing is a real error.
			if current.path == root {
				return nil, searchOutput{Error: err.Error()}, nil
			}
			continue
		}

		for _, info := range infos {
			entry := newFileEntry(current.path, info)

			matched, err := matchName(input.Pattern, entry.Name)
			if err != nil {
				return nil, searchOutput{Error: err.Error()}, nil
			}
			if matched {
				if len(matches) >= maxResults {
					truncated = true
					break
				}
				matches = append(matches, entry)
			}

			if entry.IsDir {
				queue = append(queue, struct {
					path  string
					depth int
				}{path: entry.Path, depth: current.depth + 1})
			}
		}
	}

	return nil, searchOutput{
		Matches:   matches,
		Count:     len(matches),
		Truncated: truncated,
		Success:   true,
	}, nil
}

type readInput struct {
	Path   string `json:"path" jsonschema:"file to read, relative to the root of the share"`
	Offset int    `json:"offset,omitempty" jsonschema:"first line to return, zero based"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of lines to return"`
}

type readOutput struct {
	Content    string `json:"content" jsonschema:"file content with line numbers in the format '   1| content'"`
	TotalLines int    `json:"total_lines" jsonschema:"total number of lines in the file"`
	Size       int64  `json:"size" jsonschema:"size of the file in bytes"`
	Success    bool   `json:"success" jsonschema:"whether the operation succeeded"`
	Error      string `json:"error,omitempty" jsonschema:"error message if it failed"`
}

// read pulls a text file off the share. It refuses oversized and binary files
// instead of streaming a model's context full of noise.
func (s *server) read(ctx context.Context, _ *mcp.CallToolRequest, input readInput) (
	*mcp.CallToolResult,
	readOutput,
	error,
) {
	file, err := cleanPath(input.Path)
	if err != nil {
		return nil, readOutput{Error: err.Error()}, nil
	}
	if file == "" {
		return nil, readOutput{Error: "path is the share root, which is a directory"}, nil
	}

	share, err := s.connect(ctx)
	if err != nil {
		return nil, readOutput{Error: err.Error()}, nil
	}

	info, err := share.Stat(file)
	if err != nil {
		return nil, readOutput{Error: err.Error()}, nil
	}
	if info.IsDir() {
		return nil, readOutput{Error: fmt.Sprintf("%q is a directory, use list instead", file)}, nil
	}
	if info.Size() > s.cfg.ReadMaxBytes {
		return nil, readOutput{
			Size: info.Size(),
			Error: fmt.Sprintf(
				"file is %d bytes, larger than the %d byte read limit (raise SMB_READ_MAX_BYTES to read it)",
				info.Size(), s.cfg.ReadMaxBytes,
			),
		}, nil
	}

	data, err := share.ReadFile(file)
	if err != nil {
		return nil, readOutput{Size: info.Size(), Error: err.Error()}, nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, readOutput{
			Size:  info.Size(),
			Error: fmt.Sprintf("%q looks like a binary file and was not decoded as text", file),
		}, nil
	}

	lines := splitLines(string(data))
	content := renderLines(lines, input.Offset, input.Limit)

	return nil, readOutput{
		Content:    content,
		TotalLines: len(lines),
		Size:       info.Size(),
		Success:    true,
	}, nil
}

// splitLines splits on newlines, discarding the empty final element produced
// by a trailing newline so line counts match what an editor shows.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// renderLines numbers the requested slice of lines, matching the format the
// filesystem server uses so models see one convention across both.
func renderLines(lines []string, offset, limit int) string {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return ""
	}

	end := len(lines)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	var builder strings.Builder
	for index := offset; index < end; index++ {
		fmt.Fprintf(&builder, "%4d| %s\n", index+1, lines[index])
	}
	return builder.String()
}
