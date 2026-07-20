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

// localMboxPath maps a mailbox:// folder URI to its on-disk mbox file.
// mailbox://nobody@Local%20Folders/Sub/Leaf -> <dir>/Sub.sbd/Leaf
func localMboxPath(a *Account, folderURI string) string {
	u, err := url.Parse(folderURI)
	if err != nil {
		return ""
	}
	name, _ := url.PathUnescape(strings.TrimPrefix(u.Path, "/"))
	parts := strings.Split(name, "/")
	segs := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i < len(parts)-1 {
			segs = append(segs, p+".sbd")
		} else {
			segs = append(segs, p)
		}
	}
	return filepath.Join(append([]string{a.Directory}, segs...)...)
}
