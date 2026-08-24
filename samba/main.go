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
		log.Fatalf("samba mcp: %v", err)
	}
}

func run() error {
	cfg, err := loadConfig(osGetenv)
	if err != nil {
		return err
	}

	// Stop on the signals a container runtime sends, so the SMB session is
	// logged off rather than left for the server to time out.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := newSMBClient(cfg)
	defer client.Close()

	srv := newServer(cfg, client.connect)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "samba",
		Version: "v1.0.0",
	}, nil)

	tools := srv.toolSpecs()
	for _, tool := range tools {
		tool.add(mcpServer)
	}

	log.Printf("serving share %q on %s with %d tools", cfg.Share, cfg.Address, len(tools))

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
