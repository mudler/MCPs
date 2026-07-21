package main

import (
	"context"
	"encoding/json"
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

// loadStructured reads one of the captured health_report structuredContent
// payloads from testdata and decodes it into the same `any` shape the go-sdk
// hands probeDriverAX in CallToolResult.StructuredContent.
func loadStructured(name string) any {
	GinkgoHelper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).NotTo(HaveOccurred())
	var v any
	Expect(json.Unmarshal(raw, &v)).To(Succeed())
	return v
}

var _ = Describe("axReportHasCapability", func() {
	// These three specs run against structuredContent captured verbatim from
	// cua-driver 0.9.1 inside ghcr.io/mudler/mcps/cua:latest. They are
	// behavioural assertions against real driver output, not guesses at a
	// wire shape — see .superpowers/sdd/health-report-fixture.md.
	Context("against captured driver output", func() {
		It("reports a capability when AT-SPI is working", func() {
			Expect(axReportHasCapability(loadStructured("health_report_atspi_working.json"))).To(BeTrue())
		})

		It("reports no capability when X11 is up but AT-SPI is unreachable", func() {
			Expect(axReportHasCapability(loadStructured("health_report_atspi_unreachable.json"))).To(BeFalse())
		})

		It("reports no capability when there is no DISPLAY at all", func() {
			Expect(axReportHasCapability(loadStructured("health_report_no_display.json"))).To(BeFalse())
		})

		// The two failing captures still carry overall "degraded", never
		// "failed", because ax_capability is a non-core check. Keying on
		// overall would be wrong in both directions; this pins that we do not.
		It("does not key on the envelope's overall field", func() {
			degraded := map[string]any{
				"schema_version": "1",
				"overall":        "degraded",
				"checks": []any{
					map[string]any{"name": "screen_capture_capability", "status": "fail", "message": "unrelated"},
					map[string]any{"name": "ax_capability", "status": "pass", "message": "AT-SPI is fine"},
				},
			}
			Expect(axReportHasCapability(degraded)).To(BeTrue())
		})
	})

	// Policy for the degenerate cases. The tool declares no outputSchema, so
	// anything we cannot affirmatively parse must fall to the safe side:
	// no capability, meaning pixel-only addressing plus a warning.
	Context("degenerate payloads", func() {
		It("reports no capability when structuredContent is absent", func() {
			Expect(axReportHasCapability(nil)).To(BeFalse())
		})

		It("reports no capability for a payload of the wrong shape", func() {
			Expect(axReportHasCapability("not an object")).To(BeFalse())
			Expect(axReportHasCapability(42)).To(BeFalse())
			Expect(axReportHasCapability(map[string]any{"checks": "not an array"})).To(BeFalse())
		})

		It("reports no capability when checks holds no ax_capability entry", func() {
			Expect(axReportHasCapability(map[string]any{
				"schema_version": "1",
				"overall":        "ok",
				"checks": []any{
					map[string]any{"name": "binary_version", "status": "pass", "message": "cua-driver 0.9.1"},
				},
			})).To(BeFalse())
		})

		It("reports no capability for a skipped or unrecognised ax_capability status", func() {
			for _, status := range []string{"fail", "skip", "", "unsupported"} {
				Expect(axReportHasCapability(map[string]any{
					"schema_version": "1",
					"checks": []any{
						map[string]any{"name": "ax_capability", "status": status},
					},
				})).To(BeFalse(), "status %q must not read as a working capability", status)
			}
		})
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
