package main

import (
	"bytes"
	"io"
	"strings"

	gomail "github.com/emersion/go-message/mail"
)

func ParseMessage(ref string, raw []byte) (MessageDetail, error) {
	d := MessageDetail{Ref: ref}
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return d, err
	}
	h := mr.Header
	d.Subject, _ = h.Subject()
	d.Date, _ = h.Date()
	d.MessageID = h.Get("Message-ID")
	d.References = h.Get("References")
	if from, _ := h.AddressList("From"); len(from) > 0 {
		d.Author = from[0].String()
	}
	for _, a := range mustAddrs(h, "To") {
		d.To = append(d.To, a.String())
	}
	for _, a := range mustAddrs(h, "Cc") {
		d.Cc = append(d.Cc, a.String())
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return d, err
		}
		switch ph := p.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := ph.ContentType()
			body, _ := io.ReadAll(p.Body)
			if strings.EqualFold(ct, "text/html") {
				d.BodyHTML = string(body)
			} else {
				d.BodyText += string(body)
			}
		case *gomail.AttachmentHeader:
			fn, _ := ph.Filename()
			ct, _, _ := ph.ContentType()
			body, _ := io.ReadAll(p.Body)
			d.Attachments = append(d.Attachments, Attachment{
				Filename: fn, ContentType: ct, Size: len(body),
			})
		}
	}
	return d, nil
}

func mustAddrs(h gomail.Header, key string) []*gomail.Address {
	a, _ := h.AddressList(key)
	return a
}
