package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// seed creates a file (and its parents) under root with the given content.
func seed(root, relative, content string) {
	full := filepath.Join(root, filepath.FromSlash(relative))
	Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(content), 0o644)).To(Succeed())
}

var _ = Describe("read-only tools", func() {
	var (
		root string
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		root = GinkgoT().TempDir()
		srv = newTestServer(root, nil)
		ctx = context.Background()

		seed(root, "models/qwen/model.gguf", "weights")
		seed(root, "models/qwen/config.json", `{"model_type":"qwen"}`)
		seed(root, "models/llama.gguf", "more weights")
		seed(root, "datasets/train.jsonl", "line one\nline two\nline three\n")
		Expect(os.MkdirAll(filepath.Join(root, "empty"), 0o755)).To(Succeed())
	})

	Describe("list", func() {
		It("should list the share root when no path is given", func() {
			_, out, err := srv.list(ctx, nil, listInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())

			names := []string{}
			for _, entry := range out.Entries {
				names = append(names, entry.Name)
			}
			Expect(names).To(ConsistOf("models", "datasets", "empty"))
		})

		It("should list a subdirectory without recursing", func() {
			_, out, err := srv.list(ctx, nil, listInput{Path: "models"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.Count).To(Equal(2))

			names := []string{}
			for _, entry := range out.Entries {
				names = append(names, entry.Name)
			}
			Expect(names).To(ConsistOf("qwen", "llama.gguf"))
		})

		It("should sort directories before files", func() {
			_, out, err := srv.list(ctx, nil, listInput{Path: "models"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Entries[0].Name).To(Equal("qwen"))
			Expect(out.Entries[0].IsDir).To(BeTrue())
			Expect(out.Entries[1].Name).To(Equal("llama.gguf"))
			Expect(out.Entries[1].IsDir).To(BeFalse())
		})

		It("should report size and share-relative path for each entry", func() {
			_, out, err := srv.list(ctx, nil, listInput{Path: "models/qwen"})
			Expect(err).ToNot(HaveOccurred())

			var model FileEntry
			for _, entry := range out.Entries {
				if entry.Name == "model.gguf" {
					model = entry
				}
			}
			Expect(model.Path).To(Equal("models/qwen/model.gguf"))
			Expect(model.Size).To(Equal(int64(len("weights"))))
			Expect(model.ModTime).ToNot(BeEmpty())
		})

		It("should report a missing directory as a failed result, not an error", func() {
			_, out, err := srv.list(ctx, nil, listInput{Path: "nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).ToNot(BeEmpty())
		})

		It("should refuse a path that escapes the share", func() {
			_, out, err := srv.list(ctx, nil, listInput{Path: "../.."})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("share"))
		})
	})

	Describe("search", func() {
		It("should find matches recursively by glob", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "*.gguf"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())

			paths := []string{}
			for _, match := range out.Matches {
				paths = append(paths, match.Path)
			}
			Expect(paths).To(ConsistOf("models/qwen/model.gguf", "models/llama.gguf"))
		})

		It("should search only below the given path", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "*.gguf", Path: "models/qwen"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Count).To(Equal(1))
			Expect(out.Matches[0].Path).To(Equal("models/qwen/model.gguf"))
		})

		It("should match directories too", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "qwen"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Matches).To(HaveLen(1))
			Expect(out.Matches[0].IsDir).To(BeTrue())
		})

		It("should stop at max_results and say so", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "*", MaxResults: 2})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Matches).To(HaveLen(2))
			Expect(out.Truncated).To(BeTrue())
		})

		It("should not claim truncation when everything fit", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "*.jsonl"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Truncated).To(BeFalse())
		})

		It("should not descend past max_depth", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "*.gguf", MaxDepth: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Matches).To(HaveLen(0))
		})

		It("should reject a malformed pattern", func() {
			_, out, err := srv.search(ctx, nil, searchInput{Pattern: "model[0-9"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).ToNot(BeEmpty())
		})
	})

	Describe("read", func() {
		It("should return content with line numbers", func() {
			_, out, err := srv.read(ctx, nil, readInput{Path: "datasets/train.jsonl"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.TotalLines).To(Equal(3))
			Expect(out.Content).To(ContainSubstring("   1| line one"))
			Expect(out.Content).To(ContainSubstring("   3| line three"))
		})

		It("should honour offset and limit", func() {
			_, out, err := srv.read(ctx, nil, readInput{Path: "datasets/train.jsonl", Offset: 1, Limit: 1})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Content).To(ContainSubstring("line two"))
			Expect(out.Content).ToNot(ContainSubstring("line one"))
			Expect(out.Content).ToNot(ContainSubstring("line three"))
			Expect(out.TotalLines).To(Equal(3))
		})

		It("should report the file size", func() {
			_, out, err := srv.read(ctx, nil, readInput{Path: "models/qwen/model.gguf"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Size).To(Equal(int64(len("weights"))))
		})

		It("should refuse a file larger than the read cap", func() {
			seed(root, "big.bin", strings.Repeat("x", 2048))
			small := newTestServer(root, func(cfg *Config) { cfg.ReadMaxBytes = 1024 })

			_, out, err := small.read(ctx, nil, readInput{Path: "big.bin"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("2048"))
			Expect(out.Size).To(Equal(int64(2048)))
		})

		It("should refuse a binary file rather than return mojibake", func() {
			seed(root, "weights.bin", "GGUF\x00\x00binary")

			_, out, err := srv.read(ctx, nil, readInput{Path: "weights.bin"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("binary"))
		})

		It("should report a missing file as a failed result", func() {
			_, out, err := srv.read(ctx, nil, readInput{Path: "nope.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).ToNot(BeEmpty())
		})

		It("should refuse to read a directory", func() {
			_, out, err := srv.read(ctx, nil, readInput{Path: "models"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("directory"))
		})
	})
})
