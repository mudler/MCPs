package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cleanPath", func() {
	Context("paths that stay inside the share", func() {
		DescribeTable("normalises to a share-relative slash path",
			func(input, expected string) {
				got, err := cleanPath(input)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal(expected))
			},
			Entry("empty means root", "", ""),
			Entry("dot means root", ".", ""),
			Entry("a lone slash means root", "/", ""),
			Entry("a lone backslash means root", `\`, ""),
			Entry("a single segment", "models", "models"),
			Entry("nested segments", "models/qwen/model.gguf", "models/qwen/model.gguf"),
			Entry("backslashes become slashes", `models\qwen\model.gguf`, "models/qwen/model.gguf"),
			Entry("duplicate separators collapse", "models//qwen///model.gguf", "models/qwen/model.gguf"),
			Entry("interior dots collapse", "./models/./qwen", "models/qwen"),
			Entry("a trailing slash is dropped", "models/qwen/", "models/qwen"),
			Entry("a parent hop that stays inside resolves", "models/../datasets", "datasets"),
		)
	})

	Context("paths that try to leave the share", func() {
		DescribeTable("is rejected",
			func(input string) {
				_, err := cleanPath(input)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("share"))
			},
			Entry("a bare parent hop", ".."),
			Entry("a parent hop with a tail", "../etc/passwd"),
			Entry("a parent hop hidden mid-path", "models/../../etc/passwd"),
			Entry("a backslash parent hop", `..\etc\passwd`),
			Entry("an absolute path", "/etc/passwd"),
			Entry("an absolute backslash path", `\Windows\System32`),
			Entry("a UNC path", `\\server\share\file`),
		)

		It("should reject an embedded NUL byte", func() {
			_, err := cleanPath("models/\x00/passwd")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("matchName", func() {
	Context("glob patterns", func() {
		DescribeTable("matches case-insensitively",
			func(pattern, name string, expected bool) {
				got, err := matchName(pattern, name)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal(expected))
			},
			Entry("suffix glob hits", "*.gguf", "model.gguf", true),
			Entry("suffix glob ignores case", "*.GGUF", "model.gguf", true),
			Entry("suffix glob misses", "*.gguf", "model.bin", false),
			Entry("prefix glob hits", "qwen*", "Qwen3-8B.gguf", true),
			Entry("single-character glob hits", "model?.bin", "model1.bin", true),
			Entry("character class hits", "model[0-9].bin", "model7.bin", true),
		)
	})

	Context("plain patterns without glob metacharacters", func() {
		DescribeTable("falls back to a substring match",
			func(pattern, name string, expected bool) {
				got, err := matchName(pattern, name)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal(expected))
			},
			Entry("substring in the middle", "model", "my-model-v2.bin", true),
			Entry("substring ignores case", "MODEL", "my-model-v2.bin", true),
			Entry("absent substring", "model", "dataset.bin", false),
			Entry("an empty pattern matches everything", "", "anything", true),
		)
	})

	It("should report a malformed glob rather than silently missing", func() {
		_, err := matchName("model[0-9", "model7.bin")
		Expect(err).To(HaveOccurred())
	})
})
