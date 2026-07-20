package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

func ListAccounts(_ context.Context, _ *mcp.CallToolRequest, _ ListAccountsInput) (*mcp.CallToolResult, ListAccountsOutput, error) {
	var out ListAccountsOutput
	for _, a := range app.Config.Accounts {
		info := AccountInfo{
			Key: a.Key, Type: a.Type, Name: a.Name, Hostname: a.Hostname, Local: a.IsLocal(),
		}
		for _, id := range a.Identities {
			if id.Email != "" {
				info.Identities = append(info.Identities, id.Email)
			}
		}
		out.Accounts = append(out.Accounts, info)
	}
	return nil, out, nil
}

func SearchMessages(_ context.Context, _ *mcp.CallToolRequest, in SearchMessagesInput) (*mcp.CallToolResult, SearchMessagesOutput, error) {
	since, err := parseDate(in.Since)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid since: %w", err)
	}
	until, err := parseDate(in.Until)
	if err != nil {
		return nil, SearchMessagesOutput{}, fmt.Errorf("invalid until: %w", err)
	}
	res, err := app.Gloda.Search(SearchQuery{
		Text: in.Query, From: in.From, To: in.To, Subject: in.Subject,
		FolderURI: in.Folder, Since: since, Until: until,
		Limit: in.Limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, SearchMessagesOutput{}, err
	}
	return nil, SearchMessagesOutput{Messages: res, Count: len(res)}, nil
}

func GetMessage(_ context.Context, _ *mcp.CallToolRequest, in GetMessageInput) (*mcp.CallToolResult, GetMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	var raw []byte
	if app.folderIsLocal(ref.FolderURI) {
		a, err := app.localAccountFor(ref.FolderURI)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
		mboxPath := localMboxPath(a, ref.FolderURI)
		if mboxPath == "" {
			return nil, GetMessageOutput{}, fmt.Errorf("cannot resolve a safe local folder path for %q", ref.FolderURI)
		}
		raw, err = readMboxAt(mboxPath, ref.MessageKey)
		if err != nil {
			return nil, GetMessageOutput{}, fmt.Errorf("reading local message: %w", err)
		}
	} else {
		raw, err = app.IMAP.FetchBody(ref)
		if err != nil {
			return nil, GetMessageOutput{}, err
		}
	}
	detail, err := ParseMessage(in.MessageRef, raw)
	if err != nil {
		return nil, GetMessageOutput{}, err
	}
	if in.BodyFormat != "html" {
		detail.BodyHTML = "" // keep response small unless html requested
	}
	return nil, GetMessageOutput{Message: detail}, nil
}

func ListRecent(_ context.Context, _ *mcp.CallToolRequest, in ListRecentInput) (*mcp.CallToolResult, ListRecentOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	res, err := app.Gloda.Recent(in.Folder, limit)
	if err != nil {
		return nil, ListRecentOutput{}, err
	}
	return nil, ListRecentOutput{Messages: res, Count: len(res)}, nil
}

func ListFolders(_ context.Context, _ *mcp.CallToolRequest, in ListFoldersInput) (*mcp.CallToolResult, ListFoldersOutput, error) {
	var out ListFoldersOutput
	for i := range app.Config.Accounts {
		a := &app.Config.Accounts[i]
		if in.Account != "" && a.Key != in.Account {
			continue
		}
		if a.Type == "imap" {
			fs, err := app.IMAP.listIMAPFolders(a)
			if err != nil {
				// Degrade: report the account with an error-free empty list rather than failing all.
				continue
			}
			out.Folders = append(out.Folders, fs...)
		} else {
			out.Folders = append(out.Folders, listLocalFolders(a)...)
		}
	}
	return nil, out, nil
}

func requireRemote(ref MessageRef) error {
	if app.folderIsLocal(ref.FolderURI) {
		return fmt.Errorf("folder %q is a local (POP/Local Folders) account and is read-only", ref.FolderURI)
	}
	return nil
}

func SetFlags(_ context.Context, _ *mcp.CallToolRequest, in SetFlagsInput) (*mcp.CallToolResult, SetFlagsOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, SetFlagsOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, SetFlagsOutput{}, err
	}
	var add, remove []imap.Flag
	if in.Read != nil {
		if *in.Read {
			add = append(add, imap.FlagSeen)
		} else {
			remove = append(remove, imap.FlagSeen)
		}
	}
	if in.Flagged != nil {
		if *in.Flagged {
			add = append(add, imap.FlagFlagged)
		} else {
			remove = append(remove, imap.FlagFlagged)
		}
	}
	for _, t := range in.AddTags {
		add = append(add, imap.Flag(t))
	}
	for _, t := range in.RemoveTags {
		remove = append(remove, imap.Flag(t))
	}
	if err := app.IMAP.SetFlags(ref, add, remove); err != nil {
		return nil, SetFlagsOutput{}, err
	}
	return nil, SetFlagsOutput{Success: true}, nil
}

func MoveMessage(_ context.Context, _ *mcp.CallToolRequest, in MoveMessageInput) (*mcp.CallToolResult, MoveMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, MoveMessageOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, MoveMessageOutput{}, err
	}
	if in.Destination == "" {
		return nil, MoveMessageOutput{}, fmt.Errorf("destination is required")
	}
	if err := app.IMAP.Move(ref, in.Destination); err != nil {
		return nil, MoveMessageOutput{}, err
	}
	return nil, MoveMessageOutput{Success: true}, nil
}

func DeleteMessage(_ context.Context, _ *mcp.CallToolRequest, in DeleteMessageInput) (*mcp.CallToolResult, DeleteMessageOutput, error) {
	ref, err := ParseMessageRef(in.MessageRef)
	if err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	if err := requireRemote(ref); err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	if err := app.IMAP.Delete(ref, in.Permanent); err != nil {
		return nil, DeleteMessageOutput{}, err
	}
	return nil, DeleteMessageOutput{Success: true}, nil
}

// --- Compose tools (Task 12) ---

func SendMail(_ context.Context, _ *mcp.CallToolRequest, in SendMailInput) (*mcp.CallToolResult, SendMailOutput, error) {
	if in.From == "" || len(in.To) == 0 {
		return nil, SendMailOutput{}, fmt.Errorf("from and to are required")
	}
	err := app.Sender.Send(OutgoingMessage{
		From: []string{in.From}, To: in.To, Cc: in.Cc, Bcc: in.Bcc,
		Subject: in.Subject, BodyText: in.Body,
	})
	if err != nil {
		return nil, SendMailOutput{}, err
	}
	return nil, SendMailOutput{Success: true}, nil
}

// loadDetail fetches and parses a message's full body, reusing get_message's
// local-vs-IMAP path resolution.
func loadDetail(messageRef string) (MessageDetail, error) {
	ref, err := ParseMessageRef(messageRef)
	if err != nil {
		return MessageDetail{}, err
	}
	var raw []byte
	if app.folderIsLocal(ref.FolderURI) {
		a, err := app.localAccountFor(ref.FolderURI)
		if err != nil {
			return MessageDetail{}, err
		}
		mboxPath := localMboxPath(a, ref.FolderURI)
		if mboxPath == "" {
			return MessageDetail{}, fmt.Errorf("cannot resolve a safe local folder path for %q", ref.FolderURI)
		}
		raw, err = readMboxAt(mboxPath, ref.MessageKey)
		if err != nil {
			return MessageDetail{}, err
		}
	} else {
		raw, err = app.IMAP.FetchBody(ref)
		if err != nil {
			return MessageDetail{}, err
		}
	}
	return ParseMessage(messageRef, raw)
}

func defaultFrom(_ MessageDetail, given string) string {
	if given != "" {
		return given
	}
	// Fall back to the first identity of the first account.
	for _, a := range app.Config.Accounts {
		if len(a.Identities) > 0 {
			return a.Identities[0].Email
		}
	}
	return ""
}

func ReplyMessage(_ context.Context, _ *mcp.CallToolRequest, in ReplyMessageInput) (*mcp.CallToolResult, ReplyMessageOutput, error) {
	orig, err := loadDetail(in.MessageRef)
	if err != nil {
		return nil, ReplyMessageOutput{}, err
	}
	inReplyTo, references := replyHeaders(orig)
	to := []string{firstAddress(orig.Author)}
	var cc []string
	if in.ReplyAll {
		cc = append(cc, orig.To...)
		cc = append(cc, orig.Cc...)
	}
	body := in.Body + "\n\n" + quoteForReply(orig)
	err = app.Sender.Send(OutgoingMessage{
		From: []string{defaultFrom(orig, in.From)}, To: to, Cc: cc,
		Subject: ensurePrefix(orig.Subject, "Re: "), BodyText: body,
		InReplyTo: inReplyTo, References: references,
	})
	if err != nil {
		return nil, ReplyMessageOutput{}, err
	}
	return nil, ReplyMessageOutput{Success: true}, nil
}

func ForwardMessage(_ context.Context, _ *mcp.CallToolRequest, in ForwardMessageInput) (*mcp.CallToolResult, ForwardMessageOutput, error) {
	if len(in.To) == 0 {
		return nil, ForwardMessageOutput{}, fmt.Errorf("to is required")
	}
	orig, err := loadDetail(in.MessageRef)
	if err != nil {
		return nil, ForwardMessageOutput{}, err
	}
	body := in.Body + "\n\n---------- Forwarded message ----------\n" +
		"From: " + orig.Author + "\nSubject: " + orig.Subject + "\n\n" + orig.BodyText
	err = app.Sender.Send(OutgoingMessage{
		From: []string{defaultFrom(orig, in.From)}, To: in.To,
		Subject: ensurePrefix(orig.Subject, "Fwd: "), BodyText: body,
	})
	if err != nil {
		return nil, ForwardMessageOutput{}, err
	}
	return nil, ForwardMessageOutput{Success: true}, nil
}

func SaveDraft(_ context.Context, _ *mcp.CallToolRequest, in SaveDraftInput) (*mcp.CallToolResult, SaveDraftOutput, error) {
	raw, err := BuildRFC822(OutgoingMessage{
		From: []string{in.From}, To: in.To, Subject: in.Subject, BodyText: in.Body,
	})
	if err != nil {
		return nil, SaveDraftOutput{}, err
	}
	folder := draftFolderFor(in.From)
	if err := app.IMAP.Append(folder, raw, []imap.Flag{imap.FlagDraft}); err != nil {
		return nil, SaveDraftOutput{}, err
	}
	return nil, SaveDraftOutput{Success: true}, nil
}

func firstAddress(s string) string {
	if i := strings.LastIndex(s, "<"); i >= 0 {
		if j := strings.Index(s[i:], ">"); j >= 0 {
			return s[i+1 : i+j]
		}
	}
	return strings.TrimSpace(s)
}

func ensurePrefix(subject, prefix string) string {
	if strings.HasPrefix(strings.ToLower(subject), strings.ToLower(prefix)) {
		return subject
	}
	return prefix + subject
}

// draftFolderFor returns the identity's draft folder URI, or a bare "Drafts".
func draftFolderFor(fromEmail string) string {
	for _, a := range app.Config.Accounts {
		for _, id := range a.Identities {
			if strings.EqualFold(id.Email, fromEmail) && id.DraftFolder != "" {
				return id.DraftFolder
			}
		}
	}
	return "Drafts"
}

func listLocalFolders(a *Account) []FolderInfo {
	var out []FolderInfo
	escUser := url.PathEscape(a.Username)
	escHost := url.PathEscape(a.Hostname)
	_ = filepath.Walk(a.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".msf") || strings.HasSuffix(name, ".dat") {
			return nil
		}
		// Compute the folder path relative to a.Directory and turn on-disk ".sbd"
		// intermediate directories back into plain folder segments, so the emitted
		// URI round-trips through localMboxPath to this exact file.
		rel, relErr := filepath.Rel(a.Directory, path)
		if relErr != nil {
			return nil
		}
		rawSegs := strings.Split(filepath.ToSlash(rel), "/")
		escSegs := make([]string, 0, len(rawSegs))
		for i, s := range rawSegs {
			if i < len(rawSegs)-1 {
				s = strings.TrimSuffix(s, ".sbd")
			}
			escSegs = append(escSegs, url.PathEscape(s))
		}
		out = append(out, FolderInfo{
			Name:    name,
			URI:     "mailbox://" + escUser + "@" + escHost + "/" + strings.Join(escSegs, "/"),
			Account: a.Key, Local: true,
		})
		return nil
	})
	return out
}

// --- Contacts + calendar tools (Task 13) ---

func SearchContacts(_ context.Context, _ *mcp.CallToolRequest, in SearchContactsInput) (*mcp.CallToolResult, SearchContactsOutput, error) {
	if app.Contacts == nil {
		return nil, SearchContactsOutput{}, fmt.Errorf("no address book available")
	}
	res, err := app.Contacts.Search(in.Query, in.Limit)
	if err != nil {
		return nil, SearchContactsOutput{}, err
	}
	return nil, SearchContactsOutput{Contacts: res, Count: len(res)}, nil
}

func GetContact(_ context.Context, _ *mcp.CallToolRequest, in GetContactInput) (*mcp.CallToolResult, GetContactOutput, error) {
	if app.Contacts == nil {
		return nil, GetContactOutput{}, fmt.Errorf("no address book available")
	}
	c, found, err := app.Contacts.Get(in.Email)
	if err != nil {
		return nil, GetContactOutput{}, err
	}
	return nil, GetContactOutput{Contact: c, Found: found}, nil
}

func ListCalendars(_ context.Context, _ *mcp.CallToolRequest, _ ListCalendarsInput) (*mcp.CallToolResult, ListCalendarsOutput, error) {
	return nil, ListCalendarsOutput{Calendars: app.Config.Calendars}, nil
}

func ListEvents(_ context.Context, _ *mcp.CallToolRequest, in ListEventsInput) (*mcp.CallToolResult, ListEventsOutput, error) {
	if app.Calendar == nil {
		return nil, ListEventsOutput{}, fmt.Errorf("no calendar available")
	}
	since, err := parseDate(in.Since)
	if err != nil {
		return nil, ListEventsOutput{}, fmt.Errorf("invalid since: %w", err)
	}
	until, err := parseDate(in.Until)
	if err != nil {
		return nil, ListEventsOutput{}, fmt.Errorf("invalid until: %w", err)
	}
	if since.IsZero() {
		since = fixedNow()
	}
	if until.IsZero() {
		until = since.AddDate(0, 0, 30)
	}
	res, err := app.Calendar.Events(since, until, in.Limit)
	if err != nil {
		return nil, ListEventsOutput{}, err
	}
	return nil, ListEventsOutput{Events: res, Count: len(res)}, nil
}
