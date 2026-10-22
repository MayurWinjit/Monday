package socket

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
)

// shortTempDir returns a temp directory much shorter than t.TempDir()'s
// (which embeds the full test name and can exceed the ~104-byte sockaddr_un
// path limit once "/sessions/<uuid>/monday.sock" is appended).
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mnd")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestServerClient_HappyPath(t *testing.T) {
	root := shortTempDir(t)
	server := NewServer(root)

	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	client := NewClient()
	ack, err := client.Send(Path(root, "session-1"), notifyproto.NotifyMessage{
		Type:      notifyproto.NotifyDone,
		SessionID: "session-1",
		PaneID:    "w1:p2",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !ack.OK {
		t.Fatalf("expected ok ack, got %+v", ack)
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

func TestServerClient_MultipleMessagesInOrder(t *testing.T) {
	root := shortTempDir(t)
	server := NewServer(root)
	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	client := NewClient()
	path := Path(root, "session-1")
	for i := 0; i < 3; i++ {
		paneID := []string{"w1:p1", "w1:p2", "w1:p3"}[i]
		ack, err := client.Send(path, notifyproto.NotifyMessage{Type: notifyproto.NotifyQuestion, SessionID: "session-1", PaneID: paneID})
		if err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
		if !ack.OK {
			t.Fatalf("Send %d: expected ok ack", i)
		}
	}

	seen := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case msg := <-events:
			seen = append(seen, msg.PaneID)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
	want := []string{"w1:p1", "w1:p2", "w1:p3"}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("expected events in order %v, got %v", want, seen)
		}
	}
}

func TestServer_MalformedMessageGetsRejectedAck(t *testing.T) {
	root := shortTempDir(t)
	server := NewServer(root)
	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	path := Path(root, "session-1")
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("not valid json\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var ack notifyproto.NotifyAck
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ack.OK {
		t.Fatalf("expected a rejected ack for malformed input, got %+v", ack)
	}
	if ack.Error == "" {
		t.Fatalf("expected a non-empty error message on a rejected ack")
	}

	select {
	case msg := <-events:
		t.Fatalf("expected no event to be pushed for malformed input, got %+v", msg)
	case <-time.After(200 * time.Millisecond):
		// expected: nothing arrived
	}
}

func TestServer_SessionIDMismatchGetsRejectedAck(t *testing.T) {
	root := shortTempDir(t)
	server := NewServer(root)
	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	client := NewClient()
	ack, err := client.Send(Path(root, "session-1"), notifyproto.NotifyMessage{
		Type:      notifyproto.NotifyDone,
		SessionID: "some-other-session",
		PaneID:    "w1:p2",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ack.OK {
		t.Fatalf("expected rejected ack for a session ID mismatch, got %+v", ack)
	}

	select {
	case msg := <-events:
		t.Fatalf("expected no event for a mismatched session id, got %+v", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestServer_RemovesStaleSocketBeforeListening(t *testing.T) {
	root := shortTempDir(t)
	path := Path(root, "session-1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// simulate a stale socket file left behind by a prior crashed run
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	stale.Close() // closes but the file may remain depending on platform

	server := NewServer(root)
	_, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen should recover from a stale socket file, got: %v", err)
	}
	defer closeFn()
}

func TestServer_CloseFnRemovesSocketFile(t *testing.T) {
	root := shortTempDir(t)
	server := NewServer(root)
	path := Path(root, "session-1")

	_, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn: %v", err)
	}

	client := NewClient()
	if _, err := client.Send(path, notifyproto.NotifyMessage{SessionID: "session-1"}); err == nil {
		t.Fatalf("expected Send to fail after the server closed")
	}
}

func TestClient_DialFailureOnMissingSocket(t *testing.T) {
	client := NewClient()
	_, err := client.Send("/nonexistent/path/to/monday.sock", notifyproto.NotifyMessage{SessionID: "x"})
	if err == nil {
		t.Fatalf("expected an error dialing a nonexistent socket")
	}
}
