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

func Path(root, sessionID string) string {
	return filepath.Join(root, "sessions", sessionID, "monday.sock")
}

type Server struct {
	root string
}

func NewServer(root string) *Server {
	return &Server{root: root}
}

func (s *Server) Listen(sessionID string) (<-chan notifyproto.NotifyMessage, func() error, error) {
	path := Path(s.root, sessionID)

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
	defer func() { _ = conn.Close() }()

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

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Send(socketPath string, msg notifyproto.NotifyMessage) (notifyproto.NotifyAck, error) {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: dial %s: %w", socketPath, err)
	}
	defer func() { _ = conn.Close() }()

	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: send message: %w", err)
	}

	var ack notifyproto.NotifyAck
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
		return notifyproto.NotifyAck{}, fmt.Errorf("socket: read ack: %w", err)
	}
	return ack, nil
}
