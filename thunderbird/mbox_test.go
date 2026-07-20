package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("localMboxPath", func() {
	a := &Account{Directory: "/base"}

	It("parses a real Local-Folders URI with a %20 space in the authority", func() {
		// The old url.Parse-based implementation returned "" for this URI because
		// the space/%20 in the authority made it fail to parse.
		Expect(localMboxPath(a, "mailbox://nobody@Local%20Folders/INBOX")).
			To(Equal(filepath.FromSlash("/base/INBOX")))
	})

	It("maps intermediate segments to .sbd directories", func() {
		Expect(localMboxPath(a, "mailbox://nobody@Local%20Folders/Archive/2023")).
			To(Equal(filepath.FromSlash("/base/Archive.sbd/2023")))
	})

	// Security: percent-encoded traversal must not escape a.Directory. Against
	// the pre-fix code these URIs resolved to a traversing path (e.g. /etc/passwd);
	// they must now resolve to "".
	It("rejects an encoded ../../etc/passwd traversal", func() {
		Expect(localMboxPath(a, "mailbox://nobody@Local%20Folders/%2e%2e%2f%2e%2e%2fetc%2fpasswd")).
			To(Equal(""))
	})

	It("rejects a trailing .. segment", func() {
		Expect(localMboxPath(a, "mailbox://nobody@Local%20Folders/..")).
			To(Equal(""))
	})

	It("rejects an encoded slash inside a single segment", func() {
		Expect(localMboxPath(a, "mailbox://nobody@Local%20Folders/foo%2fbar")).
			To(Equal(""))
	})
})

var _ = Describe("listLocalFolders round-trip", func() {
	It("emits URIs that resolve back through localMboxPath to existing files", func() {
		tmp := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(tmp, "INBOX"), []byte("From \n"), 0o644)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tmp, "Archive.sbd"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tmp, "Archive.sbd", "2023"), []byte("From \n"), 0o644)).To(Succeed())
		// Sidecar files that must be skipped.
		Expect(os.WriteFile(filepath.Join(tmp, "INBOX.msf"), []byte("x"), 0o644)).To(Succeed())

		a := &Account{Directory: tmp, Username: "nobody", Hostname: "Local Folders", Type: "none"}
		folders := listLocalFolders(a)
		Expect(folders).NotTo(BeEmpty())
		for _, fi := range folders {
			p := localMboxPath(a, fi.URI)
			Expect(p).To(BeAnExistingFile(), "URI %q should resolve to an existing file", fi.URI)
			Expect(p).To(HavePrefix(tmp))
		}
	})
})
