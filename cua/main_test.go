package main

import (
	"context"
	"io"
	"log"
	"log/slog"

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
