package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
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

// healthReport is the documented structuredContent payload of the driver's
// health_report tool, schema_version "1". Only the fields we key on are
// modelled; unknown fields and unknown check names are tolerated by design,
// per the tool's own stability note.
type healthReport struct {
	SchemaVersion string             `json:"schema_version"`
	Overall       string             `json:"overall"`
	Checks        []healthReportItem `json:"checks"`
}

type healthReportItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// axCheckName is the canonical name of the accessibility check. It is present
// on every platform cua-driver supports.
const axCheckName = "ax_capability"

// axReportHasCapability reports whether a health_report structuredContent
// payload affirmatively declares a working accessibility capability.
//
// The shape here is not guessed: it was captured verbatim from cua-driver
// 0.9.1 running inside ghcr.io/mudler/mcps/cua:latest. The three ground-truth
// scenarios — AT-SPI working, X11 up but AT-SPI unreachable, and no DISPLAY at
// all — together with the tool's own declared output contract are recorded in
// .superpowers/sdd/health-report-fixture.md, which also documents how to
// re-capture them against a newer driver.
//
// We key on checks[name == "ax_capability"].status == "pass" and nothing else:
//
//   - The status vocabulary is closed and small (pass | fail | skip), so an
//     allowlist over it is genuinely exhaustive.
//   - The envelope's "overall" field is deliberately NOT used. It aggregates
//     every check, so an unrelated failure drags it to "degraded" while AT-SPI
//     is perfectly fine; and because ax_capability is a non-core check it
//     never reaches "failed" on AT-SPI's account — the no-DISPLAY capture
//     reports only "degraded".
//   - The prose TextContent is unusable: all three captured ax_capability
//     messages lead with the token "X11" and the working and broken wordings
//     differ by a single conjunction.
//
// The tool declares an inputSchema but no outputSchema, so structuredContent
// is guaranteed only by its prose description. Anything this function cannot
// affirmatively parse — a nil payload, a payload that does not re-marshal or
// unmarshal, a checks array with no ax_capability entry, or any status other
// than "pass" — counts as no capability. That is the safe direction: unknown
// degrades computer_use to pixel-only addressing with a warning.
//
// It is kept pure and separately testable so a future driver change is a
// one-function fix.
func axReportHasCapability(structured any) bool {
	if structured == nil {
		return false
	}
	// StructuredContent is an `any` in go-sdk v1.4.0; round-trip it through
	// JSON rather than hand-rolling map type assertions.
	raw, err := json.Marshal(structured)
	if err != nil {
		return false
	}
	var report healthReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return false
	}
	for _, c := range report.Checks {
		if c.Name == axCheckName {
			return c.Status == "pass"
		}
	}
	return false
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

	// A failing ax_capability check is reported inside the payload with
	// isError false, so the result is read the same way either way.
	return axReportHasCapability(res.StructuredContent), nil
}
