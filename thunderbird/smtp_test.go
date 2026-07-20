package main

import (
	"io"
	"net"

	"github.com/emersion/go-smtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// captureBackend implements go-smtp's Backend/Session (v0.24.0) and records
// the DATA payload of a delivered message.
type captureBackend struct{ received string }

func (b *captureBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &captureSession{be: b}, nil
}

type captureSession struct{ be *captureBackend }

func (s *captureSession) Mail(_ string, _ *smtp.MailOptions) error { return nil }
func (s *captureSession) Rcpt(_ string, _ *smtp.RcptOptions) error { return nil }
func (s *captureSession) Data(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.be.received = string(b)
	return nil
}
func (s *captureSession) Reset()        {}
func (s *captureSession) Logout() error { return nil }

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

var _ = Describe("Sender.Send over a loopback server", func() {
	It("delivers a message", func() {
		be := &captureBackend{}
		s := smtp.NewServer(be)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer ln.Close()
		go s.Serve(ln)
		host, port, _ := net.SplitHostPort(ln.Addr().String())
		cfg := &Config{
			Accounts: []Account{{Type: "imap", Identities: []Identity{{Email: "alice@example.com", SMTPKey: "s1"}}}},
			SMTP:     map[string]SMTPServer{"s1": {Hostname: host, Port: atoi(port), SocketType: 0, Username: "alice@example.com"}},
		}
		sender := NewSender(cfg, &Credentials{byHostUser: map[string]string{credKey(host, "alice@example.com"): "pw"}})
		err = sender.Send(OutgoingMessage{From: []string{"alice@example.com"}, To: []string{"bob@example.com"}, Subject: "Hi", BodyText: "yo"})
		Expect(err).NotTo(HaveOccurred())
		Expect(be.received).To(ContainSubstring("yo"))
	})
})
