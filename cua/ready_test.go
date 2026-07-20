package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("waitForDisplay", func() {
	var ctx context.Context

	BeforeEach(func() { ctx = context.Background() })

	It("returns nil once the X socket accepts connections", func() {
		dir, err := os.MkdirTemp("", "x11-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)

		sock := filepath.Join(dir, "X99")
		ln, err := net.Listen("unix", sock)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { ln.Close() })

		Expect(waitForDisplayAt(ctx, sock, 2*time.Second)).To(Succeed())
	})

	It("times out when the socket never appears", func() {
		dir, err := os.MkdirTemp("", "x11-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)

		err = waitForDisplayAt(ctx, filepath.Join(dir, "X99"), 300*time.Millisecond)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("timed out"))
	})
})

var _ = Describe("displaySocketPath", func() {
	It("maps a display number to its abstract socket path", func() {
		Expect(displaySocketPath(":1")).To(Equal("/tmp/.X11-unix/X1"))
		Expect(displaySocketPath(":0")).To(Equal("/tmp/.X11-unix/X0"))
	})

	It("tolerates a screen suffix", func() {
		Expect(displaySocketPath(":1.0")).To(Equal("/tmp/.X11-unix/X1"))
	})

	It("returns empty for a non-local display", func() {
		Expect(displaySocketPath("host:1")).To(Equal(""))
		Expect(displaySocketPath("")).To(Equal(""))
	})
})
