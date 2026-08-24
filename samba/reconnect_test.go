package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// flakyBackend fails a fixed number of calls with a given error before
// delegating to a working backend.
type flakyBackend struct {
	failures int
	err      error
	inner    backend
	calls    int
}

func (f *flakyBackend) next() error {
	f.calls++
	if f.calls <= f.failures {
		return f.err
	}
	return nil
}

func (f *flakyBackend) Stat(name string) (os.FileInfo, error) {
	if err := f.next(); err != nil {
		return nil, err
	}
	return f.inner.Stat(name)
}

func (f *flakyBackend) ReadDir(name string) ([]os.FileInfo, error) {
	if err := f.next(); err != nil {
		return nil, err
	}
	return f.inner.ReadDir(name)
}

func (f *flakyBackend) ReadFile(name string) ([]byte, error) {
	if err := f.next(); err != nil {
		return nil, err
	}
	return f.inner.ReadFile(name)
}

func (f *flakyBackend) WriteFile(name string, data []byte, perm os.FileMode) error {
	if err := f.next(); err != nil {
		return err
	}
	return f.inner.WriteFile(name, data, perm)
}

func (f *flakyBackend) MkdirAll(name string, perm os.FileMode) error {
	if err := f.next(); err != nil {
		return err
	}
	return f.inner.MkdirAll(name, perm)
}

func (f *flakyBackend) Rename(oldpath, newpath string) error {
	if err := f.next(); err != nil {
		return err
	}
	return f.inner.Rename(oldpath, newpath)
}

func (f *flakyBackend) Remove(name string) error {
	if err := f.next(); err != nil {
		return err
	}
	return f.inner.Remove(name)
}

func (f *flakyBackend) RemoveAll(name string) error {
	if err := f.next(); err != nil {
		return err
	}
	return f.inner.RemoveAll(name)
}

var _ = Describe("isConnectionError", func() {
	DescribeTable("classifies failures",
		func(err error, expected bool) {
			Expect(isConnectionError(err)).To(Equal(expected))
		},
		Entry("a clean EOF", io.EOF, true),
		Entry("a truncated read", io.ErrUnexpectedEOF, true),
		Entry("a closed connection", net.ErrClosed, true),
		Entry("a reset peer", syscall.ECONNRESET, true),
		Entry("a broken pipe", syscall.EPIPE, true),
		Entry("a wrapped EOF", fmt.Errorf("reading response: %w", io.EOF), true),
		Entry("a missing file", os.ErrNotExist, false),
		Entry("a permission failure", os.ErrPermission, false),
		Entry("an ordinary error", errors.New("file not found"), false),
		Entry("no error at all", nil, false),
	)
})

var _ = Describe("reconnecting", func() {
	var (
		root      string
		inner     *localBackend
		reconnect int
	)

	BeforeEach(func() {
		root = GinkgoT().TempDir()
		inner = &localBackend{root: root}
		reconnect = 0
		seed(root, "notes/todo.txt", "first\n")
	})

	// provider hands out a backend whose first instance fails the given number
	// of calls; anything handed out after a reconnect is healthy, which is what
	// a real redial to a NAS that dropped an idle session looks like. It counts
	// how often a fresh instance was demanded.
	provider := func(initialFailures int, err error) func(bool) (backend, error) {
		var current *flakyBackend
		return func(fresh bool) (backend, error) {
			if fresh {
				reconnect++
				current = nil
				initialFailures = 0
			}
			if current == nil {
				current = &flakyBackend{failures: initialFailures, err: err, inner: inner}
			}
			return current, nil
		}
	}

	It("should not reconnect when the call succeeds", func() {
		wrapped := &reconnecting{get: provider(0, nil)}

		info, err := wrapped.Stat("notes/todo.txt")
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Name()).To(Equal("todo.txt"))
		Expect(reconnect).To(Equal(0))
	})

	It("should not reconnect on an ordinary filesystem error", func() {
		wrapped := &reconnecting{get: provider(0, nil)}

		_, err := wrapped.Stat("ghost.txt")
		Expect(err).To(HaveOccurred())
		Expect(reconnect).To(Equal(0))
	})

	It("should reconnect once and retry after a dropped session", func() {
		wrapped := &reconnecting{get: provider(1, io.EOF)}

		info, err := wrapped.Stat("notes/todo.txt")
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Name()).To(Equal("todo.txt"))
		Expect(reconnect).To(Equal(1))
	})

	It("should retry mutating calls too", func() {
		wrapped := &reconnecting{get: provider(1, syscall.ECONNRESET)}

		Expect(wrapped.Remove("notes/todo.txt")).To(Succeed())
		Expect(reconnect).To(Equal(1))
		Expect(exists(root, "notes/todo.txt")).To(BeFalse())
	})

	It("should give up after a single retry", func() {
		// Every instance this provider hands out is broken, standing in for a
		// server that is genuinely gone rather than merely idle-disconnected.
		alwaysBroken := func(fresh bool) (backend, error) {
			if fresh {
				reconnect++
			}
			return &flakyBackend{failures: 5, err: io.EOF, inner: inner}, nil
		}
		wrapped := &reconnecting{get: alwaysBroken}

		_, err := wrapped.Stat("notes/todo.txt")
		Expect(err).To(MatchError(io.EOF))
		Expect(reconnect).To(Equal(1))
	})

	It("should surface the original failure when reconnecting fails", func() {
		wrapped := &reconnecting{get: func(fresh bool) (backend, error) {
			if fresh {
				return nil, errors.New("dial tcp: no route to host")
			}
			return &flakyBackend{failures: 1, err: io.EOF, inner: inner}, nil
		}}

		_, err := wrapped.Stat("notes/todo.txt")
		Expect(err).To(MatchError(io.EOF))
	})
})
