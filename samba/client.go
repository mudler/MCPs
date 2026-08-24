package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	smb2 "github.com/cloudsoda/go-smb2"
)

// smbClient owns the TCP connection, the SMB session and the tree connect to
// the share. It dials lazily on the first tool call, so the server starts
// cleanly even when the NAS is asleep, and caches the session afterwards.
type smbClient struct {
	cfg Config

	mu      sync.Mutex
	conn    net.Conn
	session *smb2.Session
	share   *smb2.Share
}

func newSMBClient(cfg Config) *smbClient {
	return &smbClient{cfg: cfg}
}

// connect returns a backend for one request. The returned value redials once
// if the cached session turns out to be dead.
func (c *smbClient) connect(ctx context.Context) (backend, error) {
	get := func(fresh bool) (backend, error) {
		share, err := c.acquire(ctx, fresh)
		if err != nil {
			return nil, err
		}
		return &shareBackend{share: share.WithContext(ctx)}, nil
	}

	// Fail fast if the share cannot be reached at all, so the tool reports a
	// connection problem rather than a confusing file error.
	if _, err := get(false); err != nil {
		return nil, err
	}
	return &reconnecting{get: get}, nil
}

// acquire returns the cached share, dialling first if there is none or if the
// caller asked for a fresh one.
func (c *smbClient) acquire(ctx context.Context, fresh bool) (*smb2.Share, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fresh {
		c.closeLocked()
	}
	if c.share != nil {
		return c.share, nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	dialer := net.Dialer{Timeout: c.cfg.Timeout}
	conn, err := dialer.DialContext(dialCtx, "tcp", c.cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", c.cfg.Address, err)
	}

	smbDialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     c.cfg.User,
			Password: c.cfg.Password,
			Domain:   c.cfg.Domain,
		},
	}

	session, err := smbDialer.DialConn(dialCtx, conn, c.cfg.Address)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("authenticating to %s: %w", c.cfg.Address, err)
	}

	share, err := session.Mount(c.cfg.Share)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, fmt.Errorf("mounting share %q: %w", c.cfg.Share, err)
	}

	c.conn = conn
	c.session = session
	c.share = share

	return share, nil
}

// Close tears the session down. It is safe to call more than once.
func (c *smbClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
	return nil
}

// closeLocked releases the session, ignoring teardown errors: the caller is
// either shutting down or about to redial, and neither can act on them.
func (c *smbClient) closeLocked() {
	if c.share != nil {
		_ = c.share.Umount()
		c.share = nil
	}
	if c.session != nil {
		_ = c.session.Logoff()
		c.session = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// shareBackend adapts *smb2.Share to the backend interface. The only real work
// is translating the empty path, which means the share root here, into the "."
// that SMB expects.
type shareBackend struct {
	share *smb2.Share
}

func smbName(name string) string {
	if name == "" {
		return "."
	}
	return name
}

func (s *shareBackend) Stat(name string) (os.FileInfo, error) {
	return s.share.Stat(smbName(name))
}

func (s *shareBackend) ReadDir(name string) ([]os.FileInfo, error) {
	return s.share.ReadDir(smbName(name))
}

func (s *shareBackend) ReadFile(name string) ([]byte, error) {
	return s.share.ReadFile(smbName(name))
}

func (s *shareBackend) WriteFile(name string, data []byte, perm os.FileMode) error {
	return s.share.WriteFile(smbName(name), data, perm)
}

func (s *shareBackend) MkdirAll(name string, perm os.FileMode) error {
	return s.share.MkdirAll(smbName(name), perm)
}

func (s *shareBackend) Rename(oldpath, newpath string) error {
	return s.share.Rename(smbName(oldpath), smbName(newpath))
}

func (s *shareBackend) Remove(name string) error {
	return s.share.Remove(smbName(name))
}

func (s *shareBackend) RemoveAll(name string) error {
	return s.share.RemoveAll(smbName(name))
}
