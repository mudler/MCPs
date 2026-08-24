package main

import (
	"bytes"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The stdio transport carries JSON-RPC on stdout. Anything else printed there
// corrupts the stream, so a misconfigured server must complain on stderr only.
var _ = Describe("the binary", Label("binary"), func() {
	var binary string

	BeforeEach(func() {
		binary = filepath.Join(GinkgoT().TempDir(), "samba-mcp")
		build := exec.Command("go", "build", "-o", binary, ".")
		output, err := build.CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
	})

	It("should refuse to start without SMB_HOST, and say so on stderr only", func() {
		cmd := exec.Command(binary)
		cmd.Env = []string{"PATH=/usr/bin:/bin"}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		Expect(err).To(HaveOccurred())
		Expect(stdout.String()).To(BeEmpty())
		Expect(stderr.String()).To(ContainSubstring("SMB_HOST"))
	})

	It("should refuse to start without SMB_SHARE, and say so on stderr only", func() {
		cmd := exec.Command(binary)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "SMB_HOST=127.0.0.1"}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		Expect(err).To(HaveOccurred())
		Expect(stdout.String()).To(BeEmpty())
		Expect(stderr.String()).To(ContainSubstring("SMB_SHARE"))
	})
})
