package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// displaySocketPath maps a local DISPLAY value such as ":1" or ":1.0" to its
// Unix socket path. It returns "" for remote or malformed displays.
func displaySocketPath(display string) string {
	if !strings.HasPrefix(display, ":") {
		return ""
	}
	num := strings.TrimPrefix(display, ":")
	if i := strings.Index(num, "."); i >= 0 {
		num = num[:i]
	}
	if num == "" {
		return ""
	}
	return "/tmp/.X11-unix/X" + num
}

// waitForDisplayAt polls until the X server socket accepts a connection.
func waitForDisplayAt(ctx context.Context, sock string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for the X display socket %s: %w", timeout, sock, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// waitForDisplay waits for the X server named by DISPLAY to become reachable.
// A remote or unset DISPLAY is not something we can poll, so it is accepted
// without waiting.
func waitForDisplay(ctx context.Context, display string, timeout time.Duration) error {
	sock := displaySocketPath(display)
	if sock == "" {
		return nil
	}
	return waitForDisplayAt(ctx, sock, timeout)
}

// axReportHasCapability reports whether a cua-driver health_report body
// advertises a usable AT-SPI accessibility capability.
//
// The driver reports capabilities as text, so this is a string match rather
// than a schema decode: the exact shape of the report is not pinned by the
// driver's protocol. An absent mention and an explicit "none" both mean no
// AT-SPI. Keep this the single place that knows the report's wire shape, so
// that correcting it against a live driver is a one-function change.
func axReportHasCapability(report string) bool {
	body := strings.ToLower(report)
	if !strings.Contains(body, "ax_capability") {
		return false
	}
	return !strings.Contains(body, `"ax_capability":"none"`) &&
		!strings.Contains(body, `"ax_capability": "none"`)
}

// probeDriverAX spawns a short-lived cua-driver, asks it for a health report,
// and reports whether an accessibility capability is available. A false result
// with a nil error means the driver ran but has no AT-SPI: computer_use will
// degrade to pixel-only addressing.
func probeDriverAX(ctx context.Context, cmd string, args []string) (bool, error) {
	child := exec.CommandContext(ctx, cmd, args...)
	child.Env = append(os.Environ(), "CUA_DRIVER_RS_TELEMETRY_ENABLED=0")

	client := mcp.NewClient(&mcp.Implementation{Name: "cua-probe", Version: version}, nil)
	sess, err := client.Connect(ctx, &mcp.CommandTransport{Command: child}, nil)
	if err != nil {
		return false, fmt.Errorf("start %s: %w", cmd, err)
	}
	defer sess.Close()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "health_report"})
	if err != nil {
		return false, fmt.Errorf("health_report: %w", err)
	}

	var report strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			report.WriteString(t.Text)
		}
	}
	return axReportHasCapability(report.String()), nil
}
