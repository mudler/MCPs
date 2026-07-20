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

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// redirectNibLogsToStderr points nib's logger at stderr.
//
// nib logs through github.com/mudler/xlog, whose default logger writes to
// os.Stdout. For a stdio MCP server stdout is the JSON-RPC channel, so a single
// nib log line lands in the middle of the protocol stream and breaks the client
// before the first message is exchanged. Only the destination is overridden:
// the level still honours COGITO_LOG_LEVEL exactly as xlog's own default does.
func redirectNibLogsToStderr() {
	level := xlog.LogLevel(os.Getenv("COGITO_LOG_LEVEL")).ToSlogLevel()
	xlog.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
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

		serverT, clientT := mcp.NewInMemoryTransports()
		go func() {
			if err := nibmcp.StartComputerMCPServer(ctx, serverT, cfg.NibConfig); err != nil {
				log.Printf("computer server error: %v", err)
			}
		}()
		u, err := connectUpstream(ctx, "computer", clientT)
		if err != nil {
			return nil, err
		}
		ups = append(ups, u)
	}

	if cfg.NibConfig.Browser.Enabled {
		serverT, clientT := mcp.NewInMemoryTransports()
		go func() {
			if err := nibmcp.StartBrowserMCPServer(ctx, serverT, cfg.NibConfig); err != nil {
				log.Printf("browser server error: %v", err)
			}
		}()
		u, err := connectUpstream(ctx, "browser", clientT)
		if err != nil {
			return nil, err
		}
		ups = append(ups, u)
	}

	return ups, nil
}
