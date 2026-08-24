package main

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
)

// reconnecting wraps a backend provider so that a session dropped by the
// server, which a NAS will happily do to an idle client, costs one transparent
// redial rather than a failed tool call.
//
// get(false) returns the current backend, get(true) discards it and builds a
// fresh one.
type reconnecting struct {
	get func(fresh bool) (backend, error)
}

// retryOp runs fn against the current backend and, if it fails in a way that
// suggests the connection rather than the file, redials once and runs it
// again. Only one retry: a genuinely unreachable server should surface quickly
// instead of stalling the caller.
func retryOp[T any](r *reconnecting, fn func(backend) (T, error)) (T, error) {
	var zero T

	current, err := r.get(false)
	if err != nil {
		return zero, err
	}

	result, err := fn(current)
	if err == nil || !isConnectionError(err) {
		return result, err
	}

	fresh, freshErr := r.get(true)
	if freshErr != nil {
		// The redial failed too. The original error describes what the caller
		// was actually trying to do, so report that one.
		return zero, err
	}
	return fn(fresh)
}

// retryVoid adapts retryOp for operations that return only an error.
func retryVoid(r *reconnecting, fn func(backend) error) error {
	_, err := retryOp(r, func(b backend) (struct{}, error) { return struct{}{}, fn(b) })
	return err
}

func (r *reconnecting) Stat(name string) (os.FileInfo, error) {
	return retryOp(r, func(b backend) (os.FileInfo, error) { return b.Stat(name) })
}

func (r *reconnecting) ReadDir(name string) ([]os.FileInfo, error) {
	return retryOp(r, func(b backend) ([]os.FileInfo, error) { return b.ReadDir(name) })
}

func (r *reconnecting) ReadFile(name string) ([]byte, error) {
	return retryOp(r, func(b backend) ([]byte, error) { return b.ReadFile(name) })
}

func (r *reconnecting) WriteFile(name string, data []byte, perm os.FileMode) error {
	return retryVoid(r, func(b backend) error { return b.WriteFile(name, data, perm) })
}

func (r *reconnecting) MkdirAll(name string, perm os.FileMode) error {
	return retryVoid(r, func(b backend) error { return b.MkdirAll(name, perm) })
}

func (r *reconnecting) Rename(oldpath, newpath string) error {
	return retryVoid(r, func(b backend) error { return b.Rename(oldpath, newpath) })
}

func (r *reconnecting) Remove(name string) error {
	return retryVoid(r, func(b backend) error { return b.Remove(name) })
}

func (r *reconnecting) RemoveAll(name string) error {
	return retryVoid(r, func(b backend) error { return b.RemoveAll(name) })
}

// isConnectionError reports whether an error means the session is gone rather
// than the file. Misclassifying a missing file as a dropped connection would
// double every failed lookup, so this stays deliberately narrow.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, io.EOF),
		errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, net.ErrClosed),
		errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.ECONNABORTED),
		errors.Is(err, syscall.EPIPE),
		errors.Is(err, syscall.ETIMEDOUT):
		return true
	}

	// Deliberately not net.Error: *os.PathError implements Timeout and
	// Temporary, so matching that interface would classify every missing file
	// as a dropped session. *net.OpError only wraps a genuine network
	// operation failure.
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
