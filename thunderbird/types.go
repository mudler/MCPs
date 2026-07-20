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
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Type string `json:"type"` // "storage", "caldav", ...
	URI  string `json:"uri"`
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

// --- Tool I/O types (Task 10: read tools) ---

type ListAccountsInput struct{}
type ListAccountsOutput struct {
	Accounts []AccountInfo `json:"accounts"`
}
type AccountInfo struct {
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Hostname   string   `json:"hostname"`
	Local      bool     `json:"local" jsonschema:"true for POP/Local Folders accounts which are read-only"`
	Identities []string `json:"identities" jsonschema:"email addresses you may send as"`
}

type ListFoldersInput struct {
	Account string `json:"account,omitempty" jsonschema:"optional account key to filter (see list_accounts)"`
}
type ListFoldersOutput struct {
	Folders []FolderInfo `json:"folders"`
}
type FolderInfo struct {
	URI     string `json:"uri" jsonschema:"folder URI; used as folder filter and move destination"`
	Name    string `json:"name"`
	Account string `json:"account"`
	Unread  int    `json:"unread"`
	Total   int    `json:"total"`
	Local   bool   `json:"local"`
}

type SearchMessagesInput struct {
	Query   string `json:"query,omitempty" jsonschema:"free text matched against subject and indexed body"`
	Account string `json:"account,omitempty"`
	Folder  string `json:"folder,omitempty" jsonschema:"folder URI to restrict the search"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Subject string `json:"subject,omitempty"`
	Since   string `json:"since,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD lower bound on date"`
	Until   string `json:"until,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD upper bound on date"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
}
type SearchMessagesOutput struct {
	Messages []MessageSummary `json:"messages"`
	Count    int              `json:"count"`
}

type GetMessageInput struct {
	MessageRef string `json:"message_ref" jsonschema:"reference returned by search_messages or list_recent"`
	BodyFormat string `json:"body_format,omitempty" jsonschema:"text (default) or html"`
}
type GetMessageOutput struct {
	Message MessageDetail `json:"message"`
}

type ListRecentInput struct {
	Folder string `json:"folder,omitempty" jsonschema:"folder URI; omit for across all folders"`
	Limit  int    `json:"limit,omitempty"`
}
type ListRecentOutput struct {
	Messages []MessageSummary `json:"messages"`
	Count    int              `json:"count"`
}

// --- Tool I/O types (Task 11: mutate tools) ---

type SetFlagsInput struct {
	MessageRef string   `json:"message_ref"`
	Read       *bool    `json:"read,omitempty" jsonschema:"set read (true) or unread (false)"`
	Flagged    *bool    `json:"flagged,omitempty" jsonschema:"set or clear the star/flag"`
	AddTags    []string `json:"add_tags,omitempty"`
	RemoveTags []string `json:"remove_tags,omitempty"`
}
type SetFlagsOutput struct {
	Success bool `json:"success"`
}

type MoveMessageInput struct {
	MessageRef  string `json:"message_ref"`
	Destination string `json:"destination" jsonschema:"destination folder name within the same account (e.g. Archive)"`
}
type MoveMessageOutput struct {
	Success bool `json:"success"`
}

type DeleteMessageInput struct {
	MessageRef string `json:"message_ref"`
	Permanent  bool   `json:"permanent,omitempty" jsonschema:"if true, expunge permanently; default moves to Trash"`
}
type DeleteMessageOutput struct {
	Success bool `json:"success"`
}

// --- Tool I/O types (Task 12: compose tools) ---

type SendMailInput struct {
	From    string   `json:"from" jsonschema:"identity email to send as (see list_accounts)"`
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}
type SendMailOutput struct {
	Success bool `json:"success"`
}

type ReplyMessageInput struct {
	MessageRef string `json:"message_ref" jsonschema:"message being replied to"`
	From       string `json:"from,omitempty" jsonschema:"identity to send as; defaults to the account's identity"`
	Body       string `json:"body"`
	ReplyAll   bool   `json:"reply_all,omitempty"`
}
type ReplyMessageOutput struct {
	Success bool `json:"success"`
}

type ForwardMessageInput struct {
	MessageRef string   `json:"message_ref"`
	From       string   `json:"from,omitempty"`
	To         []string `json:"to"`
	Body       string   `json:"body,omitempty"`
}
type ForwardMessageOutput struct {
	Success bool `json:"success"`
}

type SaveDraftInput struct {
	From    string   `json:"from"`
	To      []string `json:"to,omitempty"`
	Subject string   `json:"subject,omitempty"`
	Body    string   `json:"body,omitempty"`
}
type SaveDraftOutput struct {
	Success bool `json:"success"`
}

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

// --- Contacts + calendar tools (Task 13) ---

type SearchContactsInput struct {
	Query string `json:"query" jsonschema:"name or email substring"`
	Limit int    `json:"limit,omitempty"`
}
type SearchContactsOutput struct {
	Contacts []Contact `json:"contacts"`
	Count    int       `json:"count"`
}

type GetContactInput struct {
	Email string `json:"email"`
}
type GetContactOutput struct {
	Contact Contact `json:"contact"`
	Found   bool    `json:"found"`
}

type ListCalendarsInput struct{}
type ListCalendarsOutput struct {
	Calendars []CalendarRef `json:"calendars"`
}

type ListEventsInput struct {
	Since string `json:"since,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD; defaults to now"`
	Until string `json:"until,omitempty" jsonschema:"RFC3339 or YYYY-MM-DD; defaults to 30 days out"`
	Limit int    `json:"limit,omitempty"`
}
type ListEventsOutput struct {
	Events []CalendarEvent `json:"events"`
	Count  int             `json:"count"`
}

// --- Consolidated domain tool I/O types ---

type MailInput struct {
	Action      string   `json:"action" jsonschema:"the mail operation to perform"`
	Account     string   `json:"account,omitempty" jsonschema:"list_folders: restrict to this account key"`
	Query       string   `json:"query,omitempty" jsonschema:"search_messages: free text over subject and indexed body"`
	Sender      string   `json:"sender,omitempty" jsonschema:"search_messages: filter by sender"`
	Recipient   string   `json:"recipient,omitempty" jsonschema:"search_messages: filter by recipient"`
	Subject     string   `json:"subject,omitempty" jsonschema:"search_messages: subject filter; send_mail: the subject line"`
	Folder      string   `json:"folder,omitempty" jsonschema:"search_messages/list_recent: folder URI to scope to"`
	Since       string   `json:"since,omitempty" jsonschema:"search_messages: RFC3339 or YYYY-MM-DD lower bound"`
	Until       string   `json:"until,omitempty" jsonschema:"search_messages: RFC3339 or YYYY-MM-DD upper bound"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	MessageRef  string   `json:"message_ref,omitempty" jsonschema:"get_message/set_flags/move_message/delete_message/reply_message/forward_message: reference from search_messages"`
	BodyFormat  string   `json:"body_format,omitempty" jsonschema:"get_message: text (default) or html"`
	Read        *bool    `json:"read,omitempty" jsonschema:"set_flags: mark read (true) or unread (false)"`
	Flagged     *bool    `json:"flagged,omitempty" jsonschema:"set_flags: set or clear the star"`
	AddTags     []string `json:"add_tags,omitempty" jsonschema:"set_flags: tags to add"`
	RemoveTags  []string `json:"remove_tags,omitempty" jsonschema:"set_flags: tags to remove"`
	Destination string   `json:"destination,omitempty" jsonschema:"move_message: destination folder in the same account"`
	Permanent   bool     `json:"permanent,omitempty" jsonschema:"delete_message: permanently expunge instead of moving to Trash"`
	From        string   `json:"from,omitempty" jsonschema:"send_mail/reply_message/forward_message: identity email to send as"`
	To          []string `json:"to,omitempty" jsonschema:"send_mail/forward_message: recipients"`
	Cc          []string `json:"cc,omitempty"`
	Bcc         []string `json:"bcc,omitempty"`
	Body        string   `json:"body,omitempty" jsonschema:"send_mail/reply_message/forward_message: message body"`
	ReplyAll    bool     `json:"reply_all,omitempty" jsonschema:"reply_message: reply to all recipients"`
}

type MailOutput struct {
	Accounts []AccountInfo    `json:"accounts,omitempty"`
	Folders  []FolderInfo     `json:"folders,omitempty"`
	Messages []MessageSummary `json:"messages,omitempty"`
	Message  *MessageDetail   `json:"message,omitempty"`
	Count    int              `json:"count,omitempty"`
	Success  bool             `json:"success,omitempty"`
}

type ContactsInput struct {
	Action string `json:"action" jsonschema:"search_contacts or get_contact"`
	Query  string `json:"query,omitempty" jsonschema:"search_contacts: name or email substring"`
	Email  string `json:"email,omitempty" jsonschema:"get_contact: exact email address"`
	Limit  int    `json:"limit,omitempty"`
}
type ContactsOutput struct {
	Contacts []Contact `json:"contacts,omitempty"`
	Contact  *Contact  `json:"contact,omitempty"`
	Found    bool      `json:"found,omitempty"`
	Count    int       `json:"count,omitempty"`
}

type CalendarInput struct {
	Action string `json:"action" jsonschema:"list_calendars or list_events"`
	Since  string `json:"since,omitempty" jsonschema:"list_events: RFC3339 or YYYY-MM-DD; defaults to now"`
	Until  string `json:"until,omitempty" jsonschema:"list_events: RFC3339 or YYYY-MM-DD; defaults to +30 days"`
	Limit  int    `json:"limit,omitempty"`
}
type CalendarOutput struct {
	Calendars []CalendarRef   `json:"calendars,omitempty"`
	Events    []CalendarEvent `json:"events,omitempty"`
	Count     int             `json:"count,omitempty"`
}
