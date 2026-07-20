package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("waitForDisplayAt", func() {
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

	It("returns the context error when the wait is cancelled", func() {
		dir, err := os.MkdirTemp("", "x11-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)

		cctx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		DeferCleanup(cancel)

		err = waitForDisplayAt(cctx, filepath.Join(dir, "X99"), 10*time.Second)
		Expect(err).To(MatchError(context.Canceled))
	})
})

var _ = Describe("waitForDisplay", func() {
	It("accepts a display it cannot poll without waiting", func() {
		start := time.Now()
		Expect(waitForDisplay(context.Background(), "host:1", time.Millisecond)).To(Succeed())
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	})
})

var _ = Describe("axReportHasCapability", func() {
	// These specs pin the unknown-shape policy, not any guessed driver
	// spelling: whatever the real report looks like, a body this function
	// cannot affirmatively parse must count as no capability.
	It("reports no capability for an empty body", func() {
		Expect(axReportHasCapability("")).To(BeFalse())
	})

	It("reports no capability when the field is absent entirely", func() {
		Expect(axReportHasCapability(`{"status":"ok","screens":1}`)).To(BeFalse())
	})

	It("reports no capability when the field's value does not parse", func() {
		Expect(axReportHasCapability(`{"ax_capability":}`)).To(BeFalse())
		Expect(axReportHasCapability(`ax_capability`)).To(BeFalse())
		Expect(axReportHasCapability(`{"ax_capability": 42}`)).To(BeFalse())
	})

	// Same policy applied to the value vocabulary, which is as unobserved as
	// the shape: a spelling we have not anticipated must fall to the safe
	// side rather than read as a working capability.
	It("reports no capability for an unrecognised value", func() {
		Expect(axReportHasCapability(`{"ax_capability":"unsupported"}`)).To(BeFalse())
	})
})

var _ = Describe("probeDriverAX", func() {
	It("marks a driver that cannot be started as unavailable", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		DeferCleanup(cancel)

		ok, err := probeDriverAX(ctx, "/nonexistent/cua-driver-does-not-exist", nil)
		Expect(ok).To(BeFalse())
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, errDriverUnavailable)).To(BeTrue())
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
