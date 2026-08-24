package main

import (
	"context"
	"os"
	"path/filepath"
)

// localBackend implements backend against a real directory on disk. The
// handler specs run against this rather than a mock so they exercise genuine
// filesystem semantics: real ENOENT, real "directory not empty", real renames.
// It never ships; the production backend is the SMB share.
type localBackend struct {
	root string
}

func (l *localBackend) resolve(name string) string {
	return filepath.Join(l.root, filepath.FromSlash(name))
}

func (l *localBackend) Stat(name string) (os.FileInfo, error) {
	return os.Stat(l.resolve(name))
}

func (l *localBackend) ReadDir(name string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(l.resolve(name))
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (l *localBackend) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(l.resolve(name))
}

func (l *localBackend) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(l.resolve(name), data, perm)
}

func (l *localBackend) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(l.resolve(name), perm)
}

func (l *localBackend) Rename(oldpath, newpath string) error {
	return os.Rename(l.resolve(oldpath), l.resolve(newpath))
}

func (l *localBackend) Remove(name string) error {
	return os.Remove(l.resolve(name))
}

func (l *localBackend) RemoveAll(name string) error {
	return os.RemoveAll(l.resolve(name))
}

// newTestServer wires a server against a throwaway directory and returns both,
// so specs can seed files directly and then drive the tools.
func newTestServer(root string, mutate func(*Config)) *server {
	cfg := Config{
		Address:      "test:445",
		Share:        "Data",
		Timeout:      defaultTimeout,
		ReadMaxBytes: defaultReadMaxBytes,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	b := &localBackend{root: root}
	return newServer(cfg, func(context.Context) (backend, error) { return b, nil })
}
