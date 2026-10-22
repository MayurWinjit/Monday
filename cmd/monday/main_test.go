package main

import (
	"os"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/infra/socket"
)

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mnd")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestRunNotify_HappyPath(t *testing.T) {
	home := shortTempDir(t)
	t.Setenv("HOME", home)

	server := socket.NewServer(home + "/.monday")
	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	code := runNotify([]string{"--session", "session-1", "--pane", "w1:p2", "--type", "DONE"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	select {
	case msg := <-events:
		if msg.Type != notifyproto.NotifyDone || msg.SessionID != "session-1" || msg.PaneID != "w1:p2" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestRunNotify_MissingFlags(t *testing.T) {
	if code := runNotify([]string{"--session", "x"}); code != 2 {
		t.Fatalf("expected exit code 2 for missing flags, got %d", code)
	}
}

func TestRunNotify_InvalidType(t *testing.T) {
	if code := runNotify([]string{"--session", "x", "--pane", "y", "--type", "NOPE"}); code != 2 {
		t.Fatalf("expected exit code 2 for an invalid --type, got %d", code)
	}
}

func TestRunNotify_NoSocketListening(t *testing.T) {
	home := shortTempDir(t)
	t.Setenv("HOME", home)

	code := runNotify([]string{"--session", "no-such-session", "--pane", "w1:p2", "--type", "DONE"})
	if code != 1 {
		t.Fatalf("expected exit code 1 when no server is listening, got %d", code)
	}
}
