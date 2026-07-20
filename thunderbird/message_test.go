package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseMessage", func() {
	It("extracts headers, text body and attachment metadata", func() {
		raw := "From: bob@example.com\r\n" +
			"To: alice@example.com\r\n" +
			"Subject: Report\r\n" +
			"Message-ID: <m1@example.com>\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
			"the body\r\n"
		d, err := ParseMessage("imap://h/INBOX#1", []byte(raw))
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Subject).To(Equal("Report"))
		Expect(d.Author).To(ContainSubstring("bob@example.com"))
		Expect(d.MessageID).To(Equal("<m1@example.com>"))
		Expect(d.BodyText).To(ContainSubstring("the body"))
	})
})

var _ = Describe("readMboxAt", func() {
	It("reads a single message at an offset", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "INBOX")
		content := "From - Mon Jan 1\r\nSubject: one\r\n\r\nbody one\r\n" +
			"From - Mon Jan 2\r\nSubject: two\r\n\r\nbody two\r\n"
		Expect(os.WriteFile(p, []byte(content), 0o600)).To(Succeed())
		off := uint32(len("From - Mon Jan 1\r\nSubject: one\r\n\r\nbody one\r\n"))
		raw, err := readMboxAt(p, off)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring("body two"))
		Expect(string(raw)).NotTo(ContainSubstring("body one"))
	})
})
