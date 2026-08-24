package main

import (
	"context"
	"os"
	"strings"
)

// backend is the slice of a filesystem the tools need. It mirrors the method
// set of *smb2.Share, so the production implementation is a thin adapter and
// the specs can drive the handlers against a plain directory.
type backend interface {
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(name string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
	RemoveAll(name string) error
}

// connectFunc hands back a backend bound to the request context, reconnecting
// if the previous session died.
type connectFunc func(ctx context.Context) (backend, error)

// server carries the configuration and the connection strategy shared by every
// tool handler.
type server struct {
	cfg     Config
	connect connectFunc
}

func newServer(cfg Config, connect connectFunc) *server {
	return &server{cfg: cfg, connect: connect}
}

// FileEntry describes one file or directory on the share.
type FileEntry struct {
	Name    string `json:"name" jsonschema:"entry name without any directory part"`
	Path    string `json:"path" jsonschema:"path relative to the root of the share"`
	Size    int64  `json:"size" jsonschema:"size in bytes, zero for directories"`
	ModTime string `json:"mod_time" jsonschema:"last modification time in RFC3339 format"`
	IsDir   bool   `json:"is_dir" jsonschema:"whether the entry is a directory"`
}

// newFileEntry converts a stat result into the wire representation.
func newFileEntry(parent string, info os.FileInfo) FileEntry {
	return FileEntry{
		Name:    info.Name(),
		Path:    joinPath(parent, info.Name()),
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
		IsDir:   info.IsDir(),
	}
}

// joinPath appends a name to a share-relative directory path.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// baseName returns the last segment of a share-relative path.
func baseName(p string) string {
	if index := strings.LastIndex(p, "/"); index >= 0 {
		return p[index+1:]
	}
	return p
}

// parentPath returns everything above the last segment, or "" at the root.
func parentPath(p string) string {
	if index := strings.LastIndex(p, "/"); index >= 0 {
		return p[:index]
	}
	return ""
}
