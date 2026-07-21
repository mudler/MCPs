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
		// A zero-value Config is deliberate: startUpstream substitutes the
		// default budget for a non-positive one, so this passing is evidence
		// that a zero ReadyTimeout no longer breaks startup.
		cfg := Config{}
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
			_, err := startUpstream(context.Background(), "computer", time.Minute, func(context.Context, mcp.Transport) error {
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
			_, err := startUpstream(context.Background(), "browser", time.Minute, func(context.Context, mcp.Transport) error {
				return nil
			})
			done <- err
		}()

		var err error
		Eventually(done, 10*time.Second).Should(Receive(&err))
		Expect(err.Error()).To(ContainSubstring("browser server: returned before serving"))
	})

	// CUA_READY_TIMEOUT=0 parses cleanly, so a zero budget can reach here and
	// would otherwise build an already-expired handshake deadline, failing
	// every upstream with "context deadline exceeded".
	It("substitutes the default budget for a zero timeout", func() {
		log.SetOutput(io.Discard)
		DeferCleanup(func() { log.SetOutput(GinkgoWriter) })

		type result struct {
			u   *upstream
			err error
		}
		done := make(chan result, 1)
		go func() {
			u, err := startUpstream(context.Background(), "stub", 0, func(sctx context.Context, t mcp.Transport) error {
				return mcp.NewServer(&mcp.Implementation{Name: "stub", Version: "test"}, nil).Run(sctx, t)
			})
			done <- result{u, err}
		}()

		var r result
		Eventually(done, 10*time.Second).Should(Receive(&r))
		Expect(r.err).NotTo(HaveOccurred())
		Expect(r.u).NotTo(BeNil())
		DeferCleanup(r.u.session.Close)
	})

	// The other specs all resolve through the serveErr branch. This one leaves
	// the server end unattached, so only connCtx's deadline can unblock Connect.
	It("gives up when the server never attaches, bounded by the timeout", func() {
		log.SetOutput(io.Discard)
		DeferCleanup(func() { log.SetOutput(GinkgoWriter) })

		done := make(chan error, 1)
		go func() {
			_, err := startUpstream(context.Background(), "parked", 200*time.Millisecond, func(sctx context.Context, _ mcp.Transport) error {
				<-sctx.Done()
				return sctx.Err()
			})
			done <- err
		}()

		// Generous relative to the 200ms budget but finite, so a regression
		// fails the spec rather than wedging the suite.
		var err error
		Eventually(done, 10*time.Second).Should(Receive(&err))
		Expect(err).To(MatchError(context.DeadlineExceeded))
	})
})
