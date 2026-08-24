package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Logs go to stderr: stdout carries the JSON-RPC stream and must stay
	// clean.
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	if err := run(); err != nil {
		log.Fatalf("openhab mcp: %v", err)
	}
}

func run() error {
	cfg, err := loadConfig(osLookupEnv)
	if err != nil {
		return err
	}

	client, err := newRESTClient(cfg)
	if err != nil {
		return err
	}

	// Stop on the signals a container runtime sends.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := newServer(cfg, client)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "openhab",
		Version: "v1.0.0",
	}, nil)

	tools := srv.toolSpecs()
	for _, tool := range tools {
		tool.add(mcpServer)
	}

	mode := "read-write"
	if cfg.ReadOnly {
		mode = "read-only"
	}
	if cfg.InsecureSkipVerify {
		log.Printf("warning: TLS certificate verification is disabled")
	}
	log.Printf("serving %s with %d tools (%s)", cfg.BaseURL, len(tools), mode)

	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		// A signal cancels the context; that is a clean shutdown, not a
		// failure to report.
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	return nil
}
