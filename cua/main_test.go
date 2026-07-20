package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/mudler/xlog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("startUpstreams", func() {
	var ctx context.Context

	BeforeEach(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)

		// nib and our own logger both emit during startup and at cancellation.
		// Neither is under test here, and xlog's default sink is stdout, so
		// discard both to keep the suite's output clean.
		log.SetOutput(io.Discard)
		xlog.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
		DeferCleanup(func() { log.SetOutput(GinkgoWriter) })
	})

	// The browser server is the one upstream that can be started in a bare test
	// environment: nib launches Chromium lazily on the first browser_* call, so
	// bringing the server up and listing its tools needs no browser, no X
	// display and no cua-driver. That makes it a real check that the nib
	// entrypoint is wired to a working session and exports the tools we expect.
	It("starts the browser upstream and exposes its tools", func() {
		// ReadyTimeout bounds the connect handshake in startUpstream; LoadConfig
		// always supplies a non-zero value, so a test constructing a Config by
		// hand has to as well.
		cfg := Config{ReadyTimeout: 30 * time.Second}
		cfg.NibConfig.Browser.Enabled = true

		ups, err := startUpstreams(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(ups).To(HaveLen(1))
		Expect(ups[0].name).To(Equal("browser"))

		res, err := ups[0].session.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		var names []string
		for _, tool := range res.Tools {
			names = append(names, tool.Name)
		}
		Expect(names).To(ContainElements("browser_navigate", "browser_click", "browser_snapshot"))
	})

	It("starts nothing when both upstreams are disabled", func() {
		ups, err := startUpstreams(ctx, Config{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ups).To(BeEmpty())
	})
})

var _ = Describe("startUpstream", func() {
	// nib's entrypoints can fail before they begin serving — a driver that
	// cannot be spawned, for instance. The server end of the transport pair is
	// then never connected and never closed, so the initialize round-trip has
	// nothing to answer it. Without racing the serve error against the connect,
	// this call blocks for as long as the parent context lives, which in main
	// means until the process is signalled.
	It("reports a serve function that fails before serving instead of hanging", func() {
		log.SetOutput(io.Discard)
		DeferCleanup(func() { log.SetOutput(GinkgoWriter) })

		boom := errors.New("driver could not be started")

		// The timeout is generous relative to the work (which is none) but
		// finite, so a regression fails the spec rather than wedging the suite.
		done := make(chan error, 1)
		go func() {
			_, err := startUpstream(context.Background(), "computer", time.Minute, func(mcp.Transport) error {
				return boom
			})
			done <- err
		}()

		var err error
		Eventually(done, 10*time.Second).Should(Receive(&err))
		Expect(err).To(MatchError(boom))
		Expect(err.Error()).To(ContainSubstring("computer server:"))
	})

	It("reports a serve function that returns nil before serving", func() {
		log.SetOutput(io.Discard)
		DeferCleanup(func() { log.SetOutput(GinkgoWriter) })

		done := make(chan error, 1)
		go func() {
			_, err := startUpstream(context.Background(), "browser", time.Minute, func(mcp.Transport) error {
				return nil
			})
			done <- err
		}()

		var err error
		Eventually(done, 10*time.Second).Should(Receive(&err))
		Expect(err.Error()).To(ContainSubstring("browser server: returned before serving"))
	})
})
