package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// errDriverUnavailable marks a probe failure that means the driver could not
// be started at all, as opposed to one that merely left its AT-SPI capability
// undetermined. Task 4 treats the former as fatal and the latter as a
// degradation to pixel-only addressing.
var errDriverUnavailable = errors.New("cua-driver unavailable")

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

// axCapRe matches the driver's ax_capability field across JSON and plain-text
// spellings, tolerating whitespace around the separator.
var axCapRe = regexp.MustCompile(`ax_capability"?\s*[:=]\s*"?([a-z_]+)`)

// axReportHasCapability reports whether a health_report body affirmatively
// declares a working accessibility capability.
//
// This is the one piece of unverified wire-shape knowledge in the server: it
// was written from cua-driver's source, not from a live driver's output. It is
// deliberately isolated and pure so it can be corrected against a captured
// fixture without touching the probe's process handling. Anything it cannot
// parse counts as no capability — the safe direction.
func axReportHasCapability(body string) bool {
	m := axCapRe.FindStringSubmatch(strings.ToLower(body))
	if m == nil {
		return false
	}
	switch m[1] {
	case "none", "unavailable", "null", "false", "disabled":
		return false
	default:
		return true
	}
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
		return false, fmt.Errorf("%w: start %s: %w", errDriverUnavailable, cmd, err)
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
