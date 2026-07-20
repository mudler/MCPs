package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MessageRef identifies a message by its folder URI and messageKey.
type MessageRef struct {
	FolderURI  string
	MessageKey uint32
}

// String renders the ref as "<folderURI>#<messageKey>".
func (r MessageRef) String() string {
	return fmt.Sprintf("%s#%d", r.FolderURI, r.MessageKey)
}

// ParseMessageRef parses a "<folderURI>#<messageKey>" string.
func ParseMessageRef(s string) (MessageRef, error) {
	i := strings.LastIndex(s, "#")
	if i < 0 {
		return MessageRef{}, fmt.Errorf("invalid message_ref %q", s)
	}
	key, err := strconv.ParseUint(s[i+1:], 10, 32)
	if err != nil {
		return MessageRef{}, fmt.Errorf("invalid message_ref key in %q: %w", s, err)
	}
	return MessageRef{FolderURI: s[:i], MessageKey: uint32(key)}, nil
}

// MessageSummary is a search-index result row.
type MessageSummary struct {
	Ref     string    `json:"message_ref" jsonschema:"opaque reference: pass to get_message and mutation tools"`
	Subject string    `json:"subject"`
	Author  string    `json:"author"`
	Snippet string    `json:"snippet" jsonschema:"short body excerpt from the search index (may be truncated)"`
	Date    time.Time `json:"date"`
}

// MessageDetail is a fully-fetched message (body + attachments metadata).
type MessageDetail struct {
	Ref         string       `json:"message_ref"`
	Subject     string       `json:"subject"`
	Author      string       `json:"author"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc"`
	Date        time.Time    `json:"date"`
	MessageID   string       `json:"message_id"`
	References  string       `json:"references"`
	BodyText    string       `json:"body_text"`
	BodyHTML    string       `json:"body_html,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment describes one message attachment (metadata only).
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// Config is the parsed Thunderbird profile configuration.
// Populated by loadProfile in profile.go (Task 2).
type Config struct {
	ProfileDir string
	Accounts   []Account
	SMTP       map[string]SMTPServer
	Calendars  []CalendarRef
}

// SMTPServer is one configured outgoing (SMTP) server.
type SMTPServer struct {
	Key, Hostname string
	Port          int
	SocketType    int
	AuthMethod    int
	Username      string
}

// CalendarRef is a calendar registered in the profile.
type CalendarRef struct {
	UUID string
	Name string
	Type string // "storage", "caldav", ...
	URI  string
}

// Account is one Thunderbird mail account.
type Account struct {
	Key        string // e.g. "account1"
	Type       string // "imap", "pop3", "none" (Local Folders)
	Hostname   string
	Port       int
	Username   string
	SocketType int // 0 plain, 2 STARTTLS, 3 SSL/TLS
	AuthMethod int // 10 == OAuth2
	Directory  string
	Name       string
	Identities []Identity
}

// Identity is a sending identity attached to an account.
type Identity struct {
	Key         string
	Email       string
	FullName    string
	SMTPKey     string
	DraftFolder string // folder URI
	FccFolder   string // folder URI ("sent")
}

// IsLocal reports whether the account stores mail on disk (read-only for us).
func (a Account) IsLocal() bool { return a.Type == "pop3" || a.Type == "none" }

// Contact is an address-book entry from abook.sqlite.
type Contact struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Nickname string `json:"nickname,omitempty"`
}

// CalendarEvent is a calendar item from calendar-data/local.sqlite.
type CalendarEvent struct {
	ID       string    `json:"id"`
	Calendar string    `json:"calendar"`
	Title    string    `json:"title"`
	Location string    `json:"location,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
}
