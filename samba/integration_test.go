package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs need a real SMB server. They are skipped unless SMB_TEST_ADDRESS
// points at one, and they must only ever be aimed at a throwaway server: they
// create, move and delete files. See samba/README-test.md for the one-liner
// that starts a disposable Samba container.
var _ = Describe("SMB integration", Label("integration"), func() {
	var (
		srv     *server
		client  *smbClient
		ctx     context.Context
		scratch string
	)

	BeforeEach(func() {
		address := os.Getenv("SMB_TEST_ADDRESS")
		if address == "" {
			Skip("set SMB_TEST_ADDRESS to a throwaway SMB server to run the integration specs")
		}

		cfg := Config{
			Address:      address,
			Share:        envOrDefault("SMB_TEST_SHARE", "testshare"),
			User:         envOrDefault("SMB_TEST_USER", "smbtest"),
			Password:     envOrDefault("SMB_TEST_PASSWORD", "smbtest"),
			Timeout:      defaultTimeout,
			ReadMaxBytes: defaultReadMaxBytes,
		}

		ctx = context.Background()
		client = newSMBClient(cfg)
		srv = newServer(cfg, client.connect)

		// Everything happens inside one scratch directory that this spec
		// creates and removes. Nothing outside it is ever touched.
		scratch = fmt.Sprintf("mcp-samba-test-%d", GinkgoRandomSeed())

		DeferCleanup(func() {
			_, _, _ = srv.remove(ctx, nil, deleteInput{Path: scratch, Recursive: true})
			Expect(client.Close()).To(Succeed())
		})
	})

	It("should round-trip every tool against a live share", func() {
		By("writing a file, which creates the scratch directory")
		_, written, err := srv.write(ctx, nil, writeInput{
			Path:    scratch + "/notes/todo.txt",
			Content: "first\nsecond\nthird\n",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(written.Error).To(BeEmpty())
		Expect(written.Success).To(BeTrue())

		By("listing the directory it created")
		_, listed, err := srv.list(ctx, nil, listInput{Path: scratch + "/notes"})
		Expect(err).ToNot(HaveOccurred())
		Expect(listed.Error).To(BeEmpty())
		Expect(listed.Entries).To(HaveLen(1))
		Expect(listed.Entries[0].Name).To(Equal("todo.txt"))
		Expect(listed.Entries[0].Size).To(Equal(int64(19)))
		Expect(listed.Entries[0].IsDir).To(BeFalse())
		Expect(listed.Entries[0].ModTime).ToNot(BeEmpty())

		By("listing the share root")
		_, roots, err := srv.list(ctx, nil, listInput{})
		Expect(err).ToNot(HaveOccurred())
		Expect(roots.Error).To(BeEmpty())
		rootNames := []string{}
		for _, entry := range roots.Entries {
			rootNames = append(rootNames, entry.Name)
		}
		Expect(rootNames).To(ContainElement(scratch))

		By("finding the file by glob")
		_, found, err := srv.search(ctx, nil, searchInput{Pattern: "*.txt", Path: scratch})
		Expect(err).ToNot(HaveOccurred())
		Expect(found.Error).To(BeEmpty())
		Expect(found.Matches).To(HaveLen(1))
		Expect(found.Matches[0].Path).To(Equal(scratch + "/notes/todo.txt"))

		By("reading it back with line numbers")
		_, read, err := srv.read(ctx, nil, readInput{Path: scratch + "/notes/todo.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(read.Error).To(BeEmpty())
		Expect(read.TotalLines).To(Equal(3))
		Expect(read.Content).To(ContainSubstring("   2| second"))

		By("moving it into a new directory")
		_, moved, err := srv.move(ctx, nil, moveInput{
			From: scratch + "/notes/todo.txt",
			To:   scratch + "/archive/done.txt",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(moved.Error).To(BeEmpty())

		_, gone, err := srv.read(ctx, nil, readInput{Path: scratch + "/notes/todo.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(gone.Success).To(BeFalse())

		_, arrived, err := srv.read(ctx, nil, readInput{Path: scratch + "/archive/done.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(arrived.Error).To(BeEmpty())
		Expect(arrived.TotalLines).To(Equal(3))

		By("refusing to delete a non-empty directory without recursive")
		_, refused, err := srv.remove(ctx, nil, deleteInput{Path: scratch + "/archive"})
		Expect(err).ToNot(HaveOccurred())
		Expect(refused.Success).To(BeFalse())
		Expect(refused.Error).To(ContainSubstring("recursive"))

		By("deleting the file")
		_, deleted, err := srv.remove(ctx, nil, deleteInput{Path: scratch + "/archive/done.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted.Error).To(BeEmpty())

		_, after, err := srv.list(ctx, nil, listInput{Path: scratch + "/archive"})
		Expect(err).ToNot(HaveOccurred())
		Expect(after.Entries).To(BeEmpty())
	})

	It("should refuse a path that escapes the share on a live server", func() {
		_, out, err := srv.list(ctx, nil, listInput{Path: "../../etc"})
		Expect(err).ToNot(HaveOccurred())
		Expect(out.Success).To(BeFalse())
		Expect(out.Error).To(ContainSubstring("share"))
	})

	It("should report a wrong password as a failed result rather than a panic", func() {
		cfg := Config{
			Address:      os.Getenv("SMB_TEST_ADDRESS"),
			Share:        envOrDefault("SMB_TEST_SHARE", "testshare"),
			User:         "smbtest",
			Password:     "definitely-not-the-password",
			Timeout:      defaultTimeout,
			ReadMaxBytes: defaultReadMaxBytes,
		}
		bad := newSMBClient(cfg)
		defer bad.Close()

		_, out, err := newServer(cfg, bad.connect).list(ctx, nil, listInput{})
		Expect(err).ToNot(HaveOccurred())
		Expect(out.Success).To(BeFalse())
		Expect(out.Error).ToNot(BeEmpty())
	})

	It("should read a large file only when the cap allows it", func() {
		body := strings.Repeat("x", 4096)
		_, written, err := srv.write(ctx, nil, writeInput{Path: scratch + "/big.txt", Content: body})
		Expect(err).ToNot(HaveOccurred())
		Expect(written.Error).To(BeEmpty())

		capped := newServer(Config{ReadMaxBytes: 1024}, client.connect)
		_, refused, err := capped.read(ctx, nil, readInput{Path: scratch + "/big.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(refused.Success).To(BeFalse())
		Expect(refused.Error).To(ContainSubstring("4096"))

		_, allowed, err := srv.read(ctx, nil, readInput{Path: scratch + "/big.txt"})
		Expect(err).ToNot(HaveOccurred())
		Expect(allowed.Error).To(BeEmpty())
		Expect(allowed.Size).To(Equal(int64(4096)))
	})
})

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
