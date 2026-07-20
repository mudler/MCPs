package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	nibmcp "github.com/mudler/nib/mcp"
	"github.com/mudler/xlog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is overridden at build time with -X main.version=...
var version = "dev"

func main() {
	// MCP owns stdout; every diagnostic must go to stderr.
	log.SetOutput(os.Stderr)
	log.SetPrefix("cua: ")
	redirectNibLogsToStderr()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := LoadConfig()

	if !cfg.NibConfig.Computer.Enabled && !cfg.NibConfig.Browser.Enabled {
		log.Fatal("both CUA_ENABLE_COMPUTER and CUA_ENABLE_BROWSER are false; nothing to serve")
	}

	if err := waitForDisplay(ctx, os.Getenv("DISPLAY"), cfg.ReadyTimeout); err != nil {
		log.Fatalf("desktop never became ready: %v", err)
	}

	ups, err := startUpstreams(ctx, cfg)
	if err != nil {
		log.Fatalf("startup failed: %v", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "cua", Version: version}, nil)
	if err := aggregate(ctx, srv, ups, cfg.ToolAllowlist); err != nil {
		log.Fatalf("tool aggregation failed: %v", err)
	}

	// Run returns ctx.Err() on a clean shutdown, so a SIGTERM would otherwise
	// exit non-zero and read as a crash to any supervisor.
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("server error: %v", err)
	}
}

// redirectNibLogsToStderr points nib's logger at stderr.
//
// nib logs through github.com/mudler/xlog, whose default logger writes to
// os.Stdout. For a stdio MCP server stdout is the JSON-RPC channel, so a single
// nib log line lands in the middle of the protocol stream and breaks the client
// before the first message is exchanged. Only the destination is overridden:
// the level still honours COGITO_LOG_LEVEL and the format still honours
// LOG_FORMAT, exactly as xlog's own default logger does.
func redirectNibLogsToStderr() {
	opts := &slog.HandlerOptions{Level: xlog.LogLevel(os.Getenv("COGITO_LOG_LEVEL")).ToSlogLevel()}

	// xlog compares LOG_FORMAT literally against "json" (xlog.NewLogger), so
	// match that rather than being more permissive.
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	xlog.SetLogger(slog.New(handler))
}

// startUpstream runs one nib MCP server on an in-memory transport and connects
// a client session to it.
//
// nib's entrypoints can return an error *before* they begin serving — a driver
// that fails to connect, for example. In that case the server end of the pair
// is never connected and never closed, so a bare connectUpstream would block on
// the initialize round-trip forever. Racing the serve error against the connect,
// under a timeout, turns that hang into a reported failure.
func startUpstream(ctx context.Context, name string, timeout time.Duration, serve func(context.Context, mcp.Transport) error) (*upstream, error) {
	// A non-positive budget would make the handshake deadline already-expired,
	// failing every upstream. Callers can reach here with one (CUA_READY_TIMEOUT
	// aside, the signature invites a bare duration), so defend the callee.
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}
	serverT, clientT := mcp.NewInMemoryTransports()

	// serveCtx lets us stop a server whose client never attached; otherwise a
	// failed connect would leave it running — and for the computer upstream,
	// leave a cua-driver subprocess alive — until the parent ctx ends.
	serveCtx, cancelServe := context.WithCancel(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- serve(serveCtx, serverT) }()

	// connCtx bounds the handshake only. The established session does NOT
	// inherit its cancellation: go-sdk's jsonrpc2 wraps the connection ctx in
	// notDone, stripping Done/Err. If a future SDK stops doing that, every
	// upstream would die the moment this function returns.
	connCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		u   *upstream
		err error
	}
	done := make(chan result, 1)
	go func() {
		u, err := connectUpstream(connCtx, name, clientT)
		done <- result{u, err}
	}()

	select {
	case err := <-serveErr:
		// Only a pre-serve return lands here first; a healthy server blocks in
		// Run until ctx ends.
		if err == nil {
			err = fmt.Errorf("returned before serving")
		}
		cancelServe()
		// The connect may have succeeded in the same instant; do not strand it.
		go func() {
			if r := <-done; r.err == nil && r.u.session != nil {
				r.u.session.Close()
			}
		}()
		return nil, fmt.Errorf("%s server: %w", name, err)
	case <-connCtx.Done():
		// Cancelling connCtx is not enough to unblock Connect: the in-memory
		// pair is a net.Pipe, and jsonrpc2's writer polls ctx once and then
		// writes without a deadline. A server that starts but never attaches
		// its end therefore parks the handshake for good, so enforce the budget
		// here rather than waiting on a connect that cannot return.
		cancelServe()
		go func() {
			if r := <-done; r.err == nil && r.u.session != nil {
				r.u.session.Close()
			}
		}()
		return nil, fmt.Errorf("connect to %s server: %w", name, connCtx.Err())
	case r := <-done:
		if r.err != nil {
			cancelServe()
			return nil, r.err
		}
		go func() {
			// Blocks for the server's whole life, so serveCtx is only released
			// once serving has already stopped — never cancelled out from under
			// a healthy upstream.
			err := <-serveErr
			cancelServe()
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%s server error: %v", name, err)
			}
		}()
		return r.u, nil
	}
}

// closeUpstreams releases sessions established before a later failure.
func closeUpstreams(ups []*upstream) {
	for _, u := range ups {
		if u.session != nil {
			u.session.Close()
		}
	}
}

// startUpstreams launches the enabled nib MCP servers on in-memory transports
// and connects a client session to each.
func startUpstreams(ctx context.Context, cfg Config) ([]*upstream, error) {
	var ups []*upstream

	if cfg.NibConfig.Computer.Enabled {
		// A driver we cannot spawn at all is fatal — computer_use cannot
		// function without it. But a driver that spawns and then fails the
		// health_report call is not: the tool's name and response shape are
		// the least certain part of this integration, and a version drift
		// there must not take down a working desktop. Treat it the same as
		// "no AT-SPI reported" and degrade.
		// probeDriverAX has no internal timeout — a driver that starts but
		// stalls mid-handshake would otherwise hang startup forever, so bound
		// it with the same budget as the display wait.
		probeCtx, cancelProbe := context.WithTimeout(ctx, cfg.ReadyTimeout)
		hasAX, err := probeDriverAX(probeCtx, cfg.DriverCmd, cfg.DriverArgs)
		cancelProbe()
		if err != nil {
			if errors.Is(err, errDriverUnavailable) {
				return nil, fmt.Errorf("cua-driver could not be started (CUA_DRIVER_CMD=%s): %w", cfg.DriverCmd, err)
			}
			log.Printf("WARNING: cua-driver health probe failed (%v); continuing without an AT-SPI determination", err)
			hasAX = false
		}
		if !hasAX {
			log.Print("WARNING: no AT-SPI accessibility capability reported by cua-driver. " +
				"computer_use will fall back to pixel coordinates and element indices will be unavailable. " +
				"Check that at-spi2-core is installed and a session D-Bus is running.")
		}

		u, err := startUpstream(ctx, "computer", cfg.ReadyTimeout, func(sctx context.Context, t mcp.Transport) error {
			return nibmcp.StartComputerMCPServer(sctx, t, cfg.NibConfig)
		})
		if err != nil {
			return nil, err
		}
		ups = append(ups, u)
	}

	if cfg.NibConfig.Browser.Enabled {
		u, err := startUpstream(ctx, "browser", cfg.ReadyTimeout, func(sctx context.Context, t mcp.Transport) error {
			return nibmcp.StartBrowserMCPServer(sctx, t, cfg.NibConfig)
		})
		if err != nil {
			closeUpstreams(ups)
			return nil, err
		}
		ups = append(ups, u)
	}

	return ups, nil
}
