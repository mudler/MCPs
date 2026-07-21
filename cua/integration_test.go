//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs drive the real container image over stdio, exactly as an MCP
// client would, and require Docker plus a locally present image. They are
// excluded from the normal suite by the `integration` build tag.
//
// Run with:
//
//	go test -tags integration ./cua/ -timeout 15m
//
// The load-bearing spec is the AT-SPI one. The entire rationale for driving
// cua-driver rather than the base image's own /mcp endpoint is that
// accessibility works inside this image, which is what gives computer_use real
// element indices instead of pixel-only fallback. If it ever regresses, this is
// the test that catches it. Do not weaken it to make it pass.

const (
	integrationImage     = "ghcr.io/mudler/mcps/cua:latest"
	integrationContainer = "cua-integration-test"
)

// lockedBuffer is a bytes.Buffer safe for a writer goroutine (the transport's
// tee, or exec's stderr copier) racing the spec's reads.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// dockerUnavailable returns a human-readable reason to skip, or "" when the
// environment can run these specs. Missing Docker or a missing image is an
// environmental condition, never a failure — but it is always stated, so a
// skipped run is never mistaken for a passing one.
func dockerUnavailable() string {
	if _, err := exec.LookPath("docker"); err != nil {
		return "docker is not on PATH"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil {
		return fmt.Sprintf("docker daemon is not reachable: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.CommandContext(ctx, "docker", "image", "inspect", integrationImage).CombinedOutput(); err != nil {
		return fmt.Sprintf("image %s is not present locally (build it with `make build MCP_SERVER=cua`): %s",
			integrationImage, strings.TrimSpace(string(out)))
	}
	return ""
}

// healthReportCheck mirrors one entry of the driver's health_report
// structuredContent. The full ground truth for all three AT-SPI scenarios is
// recorded in .superpowers/sdd/health-report-fixture.md.
type healthReportCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

type healthReportPayload struct {
	SchemaVersion string              `json:"schema_version"`
	Overall       string              `json:"overall"`
	Checks        []healthReportCheck `json:"checks"`
}

// computerCapture is the ComputerUseOutput shape nib returns for a capture.
type computerCapture struct {
	Summary  string `json:"summary"`
	Elements []struct {
		Index int    `json:"element_index"`
		Role  string `json:"role"`
		Label string `json:"label"`
	} `json:"elements"`
}

var _ = Describe("cua container (integration)", Ordered, func() {
	var (
		ctx     context.Context
		cancel  context.CancelFunc
		cmd     *exec.Cmd
		sess    *mcp.ClientSession
		stdout  lockedBuffer
		stderr  lockedBuffer
		started bool
	)

	BeforeAll(func() {
		if reason := dockerUnavailable(); reason != "" {
			Skip("integration specs need a working Docker and the cua image: " + reason)
		}

		// The desktop, the session bus and `cua-driver serve` all have to come
		// up before the server answers initialize, so the budget is generous.
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)

		// A stale container from an interrupted run would make --name fail.
		_ = exec.Command("docker", "rm", "-f", integrationContainer).Run()

		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-i",
			"--name", integrationContainer, integrationImage)

		stdin, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())
		pipe, err := cmd.StdoutPipe()
		Expect(err).NotTo(HaveOccurred())
		cmd.Stderr = &stderr

		Expect(cmd.Start()).To(Succeed())
		started = true

		// Tee every byte the client reads so the stdout-purity spec can audit
		// the protocol channel afterwards. mcp.CommandTransport would own the
		// pipes itself, which is why the transport is assembled by hand here.
		client := mcp.NewClient(&mcp.Implementation{Name: "cua-integration-test", Version: "v0"}, nil)
		sess, err = client.Connect(ctx, &mcp.IOTransport{
			Reader: io.NopCloser(io.TeeReader(pipe, &stdout)),
			Writer: stdin,
		}, nil)
		Expect(err).NotTo(HaveOccurred(), "container stderr:\n"+stderr.String())
	})

	AfterAll(func() {
		if sess != nil {
			sess.Close()
		}
		if started && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = exec.Command("docker", "rm", "-f", integrationContainer).Run()
		if cancel != nil {
			cancel()
		}
	})

	It("exposes the full aggregated toolset", func() {
		res, err := sess.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())

		var names []string
		for _, t := range res.Tools {
			names = append(names, t.Name)
		}
		Expect(names).To(ConsistOf(
			"computer_use",
			"browser_navigate",
			"browser_snapshot",
			"browser_click",
			"browser_type",
			"browser_press",
			"browser_scroll",
			"browser_vision",
		))
	})

	// The load-bearing fact. cua-driver's own health_report is the authority on
	// whether AT-SPI is reachable, and it is asked here the same way
	// probeDriverAX asks it — through a live `cua-driver mcp` client inside the
	// running container, which reaches the `cua-driver serve` daemon that
	// xstartup.sh launched inside the desktop session.
	It("reports a working AT-SPI capability from cua-driver's own health_report", func() {
		driver := exec.CommandContext(ctx, "docker", "exec", "-i", integrationContainer, "cua-driver", "mcp")
		var driverErr lockedBuffer
		driver.Stderr = &driverErr

		client := mcp.NewClient(&mcp.Implementation{Name: "cua-integration-probe", Version: "v0"}, nil)
		probe, err := client.Connect(ctx, &mcp.CommandTransport{Command: driver}, nil)
		Expect(err).NotTo(HaveOccurred(), "cua-driver stderr:\n"+driverErr.String())
		defer probe.Close()

		res, err := probe.CallTool(ctx, &mcp.CallToolParams{Name: "health_report"})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.StructuredContent).NotTo(BeNil(),
			"health_report returned no structuredContent; the driver's output contract has changed")

		raw, err := json.Marshal(res.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		var report healthReportPayload
		Expect(json.Unmarshal(raw, &report)).To(Succeed())

		var ax *healthReportCheck
		for i := range report.Checks {
			if report.Checks[i].Name == axCheckName {
				ax = &report.Checks[i]
			}
		}
		Expect(ax).NotTo(BeNil(),
			"no ax_capability check in health_report: "+string(raw))
		Expect(ax.Status).To(Equal("pass"),
			"AT-SPI is not working inside the image — computer_use degrades to pixel-only. "+
				"Driver says: "+ax.Message+" / hint: "+ax.Hint)

		// The production reader must agree with the raw assertion, so a change
		// to either one cannot silently diverge from the other.
		Expect(axReportHasCapability(res.StructuredContent)).To(BeTrue())
	})

	// The consequence of the fact above: a capture against the live desktop
	// must yield an addressable element tree, not an empty one. A capture with
	// no window open legitimately has nothing to inspect, so an app is launched
	// first.
	It("returns a non-empty element tree from a capture of a live window", func() {
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{
			Name:      "computer_use",
			Arguments: map[string]any{"action": "open_app", "app": "xfce4-terminal"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.IsError).To(BeFalse(), fmt.Sprintf("open_app failed: %+v", res.Content))

		// The AT-SPI tree is built lazily as the app finishes drawing, so the
		// first capture can race the window. Retry rather than sleep blindly.
		var capture computerCapture
		var text string
		var sawImage bool
		Eventually(func() int {
			capture, text, sawImage = computerCapture{}, "", false

			res, err := sess.CallTool(ctx, &mcp.CallToolParams{
				Name:      "computer_use",
				Arguments: map[string]any{"action": "capture", "app": "xfce4-terminal"},
			})
			if err != nil || res.IsError {
				return 0
			}
			for _, c := range res.Content {
				switch v := c.(type) {
				case *mcp.TextContent:
					text += v.Text
				case *mcp.ImageContent:
					sawImage = len(v.Data) > 0
				}
			}
			if res.StructuredContent == nil {
				return 0
			}
			raw, err := json.Marshal(res.StructuredContent)
			if err != nil {
				return 0
			}
			if err := json.Unmarshal(raw, &capture); err != nil {
				return 0
			}
			return len(capture.Elements)
		}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">", 0),
			"AT-SPI produced no elements for a live window: check at-spi2-core and the session D-Bus in the image")

		// Elements must be addressable, not just present: an index and a role
		// are what a model needs to act on one.
		Expect(capture.Elements[0].Role).NotTo(BeEmpty())
		Expect(capture.Summary).NotTo(ContainSubstring("no accessibility elements available"))
		Expect(text).To(ContainSubstring("Clickable elements"),
			"the element list must reach the model as text, not only as structured data")
		Expect(sawImage).To(BeTrue(), "a default (som) capture must carry a screenshot")
	})

	// An invariant the image is explicitly built to preserve: supervisord, the
	// desktop, nib's logger and cua-driver all log to stderr or to files, and
	// stdout carries nothing but the JSON-RPC stream.
	It("keeps stdout free of everything but JSON-RPC", func() {
		out := stdout.String()
		Expect(out).NotTo(BeEmpty())

		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var msg struct {
				JSONRPC string `json:"jsonrpc"`
			}
			Expect(json.Unmarshal([]byte(line), &msg)).To(Succeed(),
				"non-JSON line on the protocol channel: "+truncate(line))
			Expect(msg.JSONRPC).To(Equal("2.0"),
				"JSON line without a JSON-RPC envelope on the protocol channel: "+truncate(line))
		}

		// The diagnostics do exist — they simply went to the other channel.
		// Without this the spec above would also pass on a silent container.
		Expect(stderr.String()).To(ContainSubstring("supervisord"))
	})
})

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
