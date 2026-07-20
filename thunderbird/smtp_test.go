package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildRFC822", func() {
	It("produces a well-formed message with headers and body", func() {
		raw, err := BuildRFC822(OutgoingMessage{
			From: []string{"alice@example.com"}, To: []string{"bob@example.com"},
			Subject: "Hello", BodyText: "hi there", InReplyTo: "<msg-a@example.com>",
		})
		Expect(err).NotTo(HaveOccurred())
		s := string(raw)
		Expect(s).To(ContainSubstring("From: <alice@example.com>"))
		Expect(s).To(ContainSubstring("To: <bob@example.com>"))
		Expect(s).To(ContainSubstring("Subject: Hello"))
		Expect(s).To(ContainSubstring("In-Reply-To: <msg-a@example.com>"))
		Expect(s).To(ContainSubstring("hi there"))
	})
})

var _ = Describe("smtpForIdentity", func() {
	It("resolves the SMTP server for a From address", func() {
		cfg := &Config{
			Accounts: []Account{{Type: "imap", Identities: []Identity{{Email: "alice@example.com", SMTPKey: "smtp1"}}}},
			SMTP:     map[string]SMTPServer{"smtp1": {Hostname: "smtp.example.com", Port: 587, SocketType: 2, Username: "alice@example.com"}},
		}
		s := NewSender(cfg, &Credentials{})
		srv, id, _, err := s.smtpForIdentity("alice@example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(srv.Hostname).To(Equal("smtp.example.com"))
		Expect(id.Email).To(Equal("alice@example.com"))
	})
	It("errors on an unknown From", func() {
		s := NewSender(&Config{}, &Credentials{})
		_, _, _, err := s.smtpForIdentity("nobody@nowhere")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("quoteForReply", func() {
	It("prefixes each line with '> '", func() {
		q := quoteForReply(MessageDetail{Author: "bob@example.com", BodyText: "line1\nline2"})
		Expect(q).To(ContainSubstring("> line1"))
		Expect(q).To(ContainSubstring("> line2"))
		Expect(q).To(ContainSubstring("bob@example.com"))
	})
})
