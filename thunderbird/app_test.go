package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoadApp (profile only)", func() {
	It("loads config and degrades gracefully when data sources are absent", func() {
		// testdata dir has prefs.js but no gloda/abook/calendar/key4 dbs.
		a, err := loadAppFromConfig(&Config{ProfileDir: "testdata", Accounts: nil, SMTP: map[string]SMTPServer{}}, &Credentials{}, false, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(a.Gloda).To(BeNil())    // no global-messages-db.sqlite in testdata
		Expect(a.Contacts).To(BeNil()) // no abook.sqlite
		Expect(a.IMAP).NotTo(BeNil())  // always constructed
	})
})

var _ = Describe("folderIsLocal", func() {
	It("treats mailbox:// as local and imap:// as remote", func() {
		a := &App{Config: &Config{Accounts: []Account{
			{Type: "imap", Hostname: "imap.example.com"},
			{Type: "none", Hostname: "Local Folders"},
		}}}
		Expect(a.folderIsLocal("mailbox://nobody@Local%20Folders/INBOX")).To(BeTrue())
		Expect(a.folderIsLocal("imap://alice@imap.example.com/INBOX")).To(BeFalse())
	})
})
