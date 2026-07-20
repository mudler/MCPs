package main

import (
	"bufio"
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// mailboxFolderPath extracts the folder path from a mailbox:// URI without
// url.Parse (which fails on the space/%20 in real Local-Folders authorities like
// "mailbox://nobody@Local%20Folders/INBOX"). It strips the scheme, drops the
// user@host authority (everything up to and including the first '/'), and returns
// the percent-decoded, '/'-separated folder segments.
func mailboxFolderPath(folderURI string) []string {
	rest := strings.TrimPrefix(folderURI, "mailbox://")
	// Drop authority: everything up to and including the first '/'.
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[i+1:]
	} else {
		rest = ""
	}
	if rest == "" {
		return nil
	}
	parts := strings.Split(rest, "/")
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		dec, err := url.PathUnescape(p)
		if err != nil {
			dec = p
		}
		segs = append(segs, dec)
	}
	return segs
}

// mailboxAuthorityHost returns the percent-decoded host portion (after '@') of a
// mailbox:// URI authority, e.g. "Local Folders" for
// "mailbox://nobody@Local%20Folders/INBOX".
func mailboxAuthorityHost(folderURI string) string {
	rest := strings.TrimPrefix(folderURI, "mailbox://")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		rest = rest[at+1:]
	}
	if dec, err := url.PathUnescape(rest); err == nil {
		return dec
	}
	return rest
}

// readMboxAt returns the raw bytes of one mbox message beginning at offset,
// stopping before the next "From " separator line or at EOF.
func readMboxAt(mboxPath string, offset uint32) ([]byte, error) {
	f, err := os.Open(mboxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	var out bytes.Buffer
	first := true
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			// A "From " at the start of a line (after the first) marks the next message.
			if !first && (bytes.HasPrefix(line, []byte("From ")) || bytes.HasPrefix(line, []byte("From - "))) {
				break
			}
			// Skip the leading separator line of this message.
			if !(first && (bytes.HasPrefix(line, []byte("From ")) || bytes.HasPrefix(line, []byte("From - ")))) {
				out.Write(line)
			}
			first = false
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// unsafeFolderSegment reports whether a decoded folder segment is unsafe to use
// as a single path component. Rejecting these closes a path-traversal hole where
// a percent-encoded slash or "..", surviving inside a segment after decoding,
// would otherwise let filepath.Join escape a.Directory.
func unsafeFolderSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return true
	}
	if strings.ContainsRune(s, '\x00') {
		return true
	}
	return strings.ContainsRune(s, '/') || strings.ContainsRune(s, os.PathSeparator)
}

// localMboxPath maps a mailbox:// folder URI to its on-disk mbox file.
// mailbox://nobody@Local%20Folders/Sub/Leaf -> <dir>/Sub.sbd/Leaf
// It returns "" when the URI cannot be resolved to a path that is provably
// inside a.Directory (defense in depth against path traversal).
func localMboxPath(a *Account, folderURI string) string {
	parts := mailboxFolderPath(folderURI)
	if len(parts) == 0 {
		return a.Directory
	}
	segs := make([]string, 0, len(parts))
	for i, p := range parts {
		if unsafeFolderSegment(p) {
			return ""
		}
		if i < len(parts)-1 {
			segs = append(segs, p+".sbd")
		} else {
			segs = append(segs, p)
		}
	}
	joined := filepath.Join(append([]string{a.Directory}, segs...)...)
	// Backstop: confirm the joined path stays within a.Directory.
	rel, err := filepath.Rel(a.Directory, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return ""
	}
	return joined
}
