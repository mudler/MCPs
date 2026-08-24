package main

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// onDisk reads a seeded file back through the real filesystem, so assertions
// check what actually landed rather than what the handler claims.
func onDisk(root, relative string) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}

func exists(root, relative string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil
}

var _ = Describe("mutating tools", func() {
	var (
		root string
		srv  *server
		ctx  context.Context
	)

	BeforeEach(func() {
		root = GinkgoT().TempDir()
		srv = newTestServer(root, nil)
		ctx = context.Background()

		seed(root, "notes/todo.txt", "first\n")
		seed(root, "notes/done.txt", "shipped\n")
		Expect(os.MkdirAll(filepath.Join(root, "empty"), 0o755)).To(Succeed())
	})

	Describe("write", func() {
		It("should create a file and its parent directories", func() {
			_, out, err := srv.write(ctx, nil, writeInput{Path: "deep/nested/new.txt", Content: "hello"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(out.BytesWritten).To(Equal(5))
			Expect(onDisk(root, "deep/nested/new.txt")).To(Equal("hello"))
		})

		It("should overwrite an existing file by default", func() {
			_, out, err := srv.write(ctx, nil, writeInput{Path: "notes/todo.txt", Content: "replaced"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(onDisk(root, "notes/todo.txt")).To(Equal("replaced"))
		})

		It("should refuse to write over a directory", func() {
			_, out, err := srv.write(ctx, nil, writeInput{Path: "notes", Content: "nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("directory"))
		})

		It("should refuse a path that escapes the share", func() {
			_, out, err := srv.write(ctx, nil, writeInput{Path: "../escape.txt", Content: "nope"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(exists(root, "../escape.txt")).To(BeFalse())
		})

		Context("when the server is read-only", func() {
			BeforeEach(func() {
				srv = newTestServer(root, func(cfg *Config) { cfg.ReadOnly = true })
			})

			It("should refuse the write", func() {
				_, out, err := srv.write(ctx, nil, writeInput{Path: "notes/todo.txt", Content: "replaced"})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeFalse())
				Expect(out.Error).To(ContainSubstring("SMB_READ_ONLY"))
				Expect(onDisk(root, "notes/todo.txt")).To(Equal("first\n"))
			})
		})

		Context("when deletion is disabled", func() {
			BeforeEach(func() {
				srv = newTestServer(root, func(cfg *Config) { cfg.DisableDelete = true })
			})

			It("should still create a new file", func() {
				_, out, err := srv.write(ctx, nil, writeInput{Path: "notes/fresh.txt", Content: "new"})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeTrue())
				Expect(onDisk(root, "notes/fresh.txt")).To(Equal("new"))
			})

			It("should refuse to overwrite an existing file", func() {
				_, out, err := srv.write(ctx, nil, writeInput{Path: "notes/todo.txt", Content: "replaced"})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeFalse())
				Expect(out.Error).To(ContainSubstring("SMB_DISABLE_DELETE"))
				Expect(onDisk(root, "notes/todo.txt")).To(Equal("first\n"))
			})
		})
	})

	Describe("move", func() {
		It("should rename a file", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/later.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(exists(root, "notes/todo.txt")).To(BeFalse())
			Expect(onDisk(root, "notes/later.txt")).To(Equal("first\n"))
		})

		It("should create missing parent directories of the destination", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "archive/2026/todo.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(onDisk(root, "archive/2026/todo.txt")).To(Equal("first\n"))
		})

		It("should refuse to clobber an existing destination", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/done.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("overwrite"))
			Expect(onDisk(root, "notes/done.txt")).To(Equal("shipped\n"))
		})

		It("should clobber when overwrite is requested", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/done.txt", Overwrite: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(onDisk(root, "notes/done.txt")).To(Equal("first\n"))
		})

		It("should report a missing source as a failed result", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "notes/ghost.txt", To: "notes/later.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).ToNot(BeEmpty())
		})

		It("should refuse to move the share root", func() {
			_, out, err := srv.move(ctx, nil, moveInput{From: "", To: "somewhere"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
		})

		Context("when the server is read-only", func() {
			BeforeEach(func() {
				srv = newTestServer(root, func(cfg *Config) { cfg.ReadOnly = true })
			})

			It("should refuse the move", func() {
				_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/later.txt"})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeFalse())
				Expect(out.Error).To(ContainSubstring("SMB_READ_ONLY"))
				Expect(exists(root, "notes/todo.txt")).To(BeTrue())
			})
		})

		Context("when deletion is disabled", func() {
			BeforeEach(func() {
				srv = newTestServer(root, func(cfg *Config) { cfg.DisableDelete = true })
			})

			It("should still move onto a free destination", func() {
				_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/later.txt"})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeTrue())
				Expect(onDisk(root, "notes/later.txt")).To(Equal("first\n"))
			})

			It("should refuse an overwriting move", func() {
				_, out, err := srv.move(ctx, nil, moveInput{From: "notes/todo.txt", To: "notes/done.txt", Overwrite: true})
				Expect(err).ToNot(HaveOccurred())
				Expect(out.Success).To(BeFalse())
				Expect(out.Error).To(ContainSubstring("SMB_DISABLE_DELETE"))
				Expect(onDisk(root, "notes/done.txt")).To(Equal("shipped\n"))
			})
		})
	})

	Describe("delete", func() {
		It("should remove a file", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "notes/todo.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(exists(root, "notes/todo.txt")).To(BeFalse())
		})

		It("should remove an empty directory", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "empty"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(exists(root, "empty")).To(BeFalse())
		})

		It("should refuse a non-empty directory without recursive", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "notes"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("recursive"))
			Expect(exists(root, "notes/todo.txt")).To(BeTrue())
		})

		It("should remove a non-empty directory when recursive is set", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "notes", Recursive: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeTrue())
			Expect(exists(root, "notes")).To(BeFalse())
		})

		It("should refuse to delete the share root", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "", Recursive: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("root"))
			Expect(exists(root, "notes")).To(BeTrue())
		})

		It("should report a missing path as a failed result", func() {
			_, out, err := srv.remove(ctx, nil, deleteInput{Path: "ghost.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).ToNot(BeEmpty())
		})

		It("should refuse when the server is read-only", func() {
			readOnly := newTestServer(root, func(cfg *Config) { cfg.ReadOnly = true })
			_, out, err := readOnly.remove(ctx, nil, deleteInput{Path: "notes/todo.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("SMB_READ_ONLY"))
			Expect(exists(root, "notes/todo.txt")).To(BeTrue())
		})

		It("should refuse when deletion is disabled", func() {
			noDelete := newTestServer(root, func(cfg *Config) { cfg.DisableDelete = true })
			_, out, err := noDelete.remove(ctx, nil, deleteInput{Path: "notes/todo.txt"})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Success).To(BeFalse())
			Expect(out.Error).To(ContainSubstring("SMB_DISABLE_DELETE"))
			Expect(exists(root, "notes/todo.txt")).To(BeTrue())
		})
	})
})

var _ = Describe("toolSpecs", func() {
	specNames := func(cfg Config) []string {
		srv := newServer(cfg, nil)
		names := []string{}
		for _, spec := range srv.toolSpecs() {
			names = append(names, spec.name)
		}
		return names
	}

	// The prefix keeps these names from colliding with the filesystem server,
	// which also registers read and write.
	samba := Config{ToolPrefix: "samba_"}

	It("should expose every tool, prefixed", func() {
		Expect(specNames(samba)).To(Equal([]string{
			"samba_list", "samba_search", "samba_read",
			"samba_write", "samba_move", "samba_delete",
		}))
	})

	It("should expose only the read-only tools when SMB_READ_ONLY is set", func() {
		cfg := samba
		cfg.ReadOnly = true
		Expect(specNames(cfg)).To(Equal([]string{"samba_list", "samba_search", "samba_read"}))
	})

	It("should drop only delete when SMB_DISABLE_DELETE is set", func() {
		cfg := samba
		cfg.DisableDelete = true
		Expect(specNames(cfg)).To(Equal([]string{
			"samba_list", "samba_search", "samba_read", "samba_write", "samba_move",
		}))
	})

	It("should stay read-only when both switches are set", func() {
		cfg := samba
		cfg.ReadOnly = true
		cfg.DisableDelete = true
		Expect(specNames(cfg)).To(Equal([]string{"samba_list", "samba_search", "samba_read"}))
	})

	It("should apply a custom prefix to every tool", func() {
		Expect(specNames(Config{ToolPrefix: "nas1_"})).To(Equal([]string{
			"nas1_list", "nas1_search", "nas1_read",
			"nas1_write", "nas1_move", "nas1_delete",
		}))
	})

	It("should leave the names bare when the prefix is empty", func() {
		Expect(specNames(Config{})).To(Equal([]string{
			"list", "search", "read", "write", "move", "delete",
		}))
	})
})
