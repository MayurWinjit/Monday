// Package socket implements the Unix domain socket notify protocol from
// architect-plan section 5.3: one NotifyMessage per connection - the client
// dials, writes one newline-terminated JSON object, reads one
// newline-terminated NotifyAck JSON object, then closes. No multiplexing,
// no length-prefixing, no keep-alive.
package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/MayurWinjit/monday/internal/domain"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
)

var (
	_ domain.NotifyServer = (*Server)(nil)
	_ domain.NotifyClient = (*Client)(nil)
)

// Path returns the socket path for a session: <root>/sessions/<id>/monday.sock,
// where root is ~/.monday. Exported so the `monday notify` subcommand (a
// separate process from the TUI) can compute it from --session alone,
// without depending on infra/fsstore or any inherited environment variable
// - see architect-plan section 11.
//
// Unix domain socket paths are limited to roughly 104 (macOS) to 108
// (Linux) bytes including the trailing NUL. A deeply nested home directory
// combined with a 36-character session UUID could theoretically approach
// that limit; this is a known, accepted constraint of the design (not
// worked around here) since ~/.monday/sessions/<uuid>/monday.sock comfortably
// fits for any realistic home directory depth.
func Path(root, sessionID string) string {
	return filepath.Join(root, "sessions", sessionID, "monday.sock")
}

// Server implements domain.NotifyServer. root is ~/.monday.
type Server struct {
	root string
}

func NewServer(root string) *Server {
	return &Server{root: root}
}

// Listen opens the session's socket (removing any stale file left behind
// by a prior crashed run first, per challenge #7) and returns a channel
// that receives one NotifyMessage per accepted connection, plus a closeFn
// that stops accepting and removes the socket file.
//
// The accept loop and its per-connection goroutines never touch the
// filesystem beyond this socket and never mutate domain state directly -
// they only push a decoded NotifyMessage onto events. All domain mutation
// happens in the caller's single-threaded event loop (the TUI's Update),
// which is the single-writer rule from architect-plan section 5.3.
func (s *Server) Listen(sessionID string) (<-chan notifyproto.NotifyMessage, func() error, error) {
	path := Path(s.root, sessionID)

	// The session directory itself is normally already created by
	// fsstore.CreateSessionDir before Listen is called, but ensure it here
	// too - a missing parent directory should not be a surprising way for
	// ListenUnix to fail.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("socket: ensure session dir for %s: %w", path, err)
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("socket: remove stale socket %s: %w", path, err)
	}

	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, nil, fmt.Errorf("socket: listen %s: %w", path, err)
	}

	events := make(chan notifyproto.NotifyMessage, 16)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Either Close() below was called (expected shutdown) or a
				// fatal listener error - either way, stop accepting.
				return
			}
			go handleConn(conn, sessionID, events)
		}
	}()

	closeFn := func() error {
		err := ln.Close()
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			if err == nil {
				err = removeErr
			}
		}
		return err
	}

	return events, closeFn, nil
}

func handleConn(conn net.Conn, sessionID string, events chan<- notifyproto.NotifyMessage) {
	defer conn.Close()

	var msg notifyproto.NotifyMessage
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		writeAck(conn, notifyproto.NotifyAck{OK: false, Error: fmt.Sprintf("malformed message: %v", err)})
		return
	}
	if msg.SessionID != sessionID {
		writeAck(conn, notifyproto.NotifyAck{OK: false, Error: fmt.Sprintf("session id mismatch: this socket is for %s, got %s", sessionID, msg.SessionID)})
		return
	}

	events <- msg
	writeAck(conn, notifyproto.NotifyAck{OK: true})
}

func writeAck(conn net.Conn, ack notifyproto.NotifyAck) {
	_ = json.NewEncoder(conn).Encode(ack)
}

// Client implements domain.NotifyClient, used by the `monday notify`
// subcommand - a separate process invocation from the long-running TUI.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Send(socketPath string, msg notifyproto.NotifyMessage) (notifyproto.NotifyAck, error) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: dial %s: %w", socketPath, err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: send message: %w", err)
	}

	var ack notifyproto.NotifyAck
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: read ack: %w", err)
	}
	return ack, nil
}
