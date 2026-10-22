package toolcheck

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeHerdr struct {
	err error
}

func (f fakeHerdr) CurrentPaneID() (string, error) {
	return "w1:p1", f.err
}

func lookPathAllFound(string) (string, error) { return "/usr/bin/x", nil }

func lookPathMissing(missing string) func(string) (string, error) {
	return func(name string) (string, error) {
		if name == missing {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
}

func TestRun_AllOK(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, t.TempDir(), lookPathAllFound)
	res := c.Run()
	if !res.OK() {
		t.Fatalf("expected OK, got %+v", res)
	}
}

func TestRun_MissingHerdr(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, t.TempDir(), lookPathMissing("herdr"))
	res := c.Run()
	if !res.MissingHerdr {
		t.Fatalf("expected MissingHerdr, got %+v", res)
	}
	if res.OK() {
		t.Fatalf("expected not OK")
	}
}

func TestRun_MissingClaude(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, t.TempDir(), lookPathMissing("claude"))
	res := c.Run()
	if !res.MissingClaude {
		t.Fatalf("expected MissingClaude, got %+v", res)
	}
}

func TestRun_NotInHerdrPane(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{err: errors.New("no current pane")}, t.TempDir(), lookPathAllFound)
	res := c.Run()
	if !res.NotInHerdrPane {
		t.Fatalf("expected NotInHerdrPane, got %+v", res)
	}
}

func TestRun_MissingHerdrSkipsPaneCheck(t *testing.T) {
	// herdr missing means CurrentPaneID would exec a nonexistent binary;
	// Run should not report NotInHerdrPane as a second, redundant failure.
	c := NewWithLookPath(fakeHerdr{err: errors.New("should not matter")}, t.TempDir(), lookPathMissing("herdr"))
	res := c.Run()
	if res.NotInHerdrPane {
		t.Fatalf("expected NotInHerdrPane to be skipped when herdr itself is missing, got %+v", res)
	}
}

func writeSession(t *testing.T, homeDir, id, status string) {
	t.Helper()
	dir := filepath.Join(homeDir, ".monday", "sessions", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `{"id":"` + id + `","status":"` + status + `"}`
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write session.json: %v", err)
	}
}

func TestRun_NoRunningSessions(t *testing.T) {
	home := t.TempDir()
	writeSession(t, home, "s1", "done")
	writeSession(t, home, "s2", "failed")

	c := NewWithLookPath(fakeHerdr{}, home, lookPathAllFound)
	res := c.Run()
	if res.AlreadyRunningID != "" {
		t.Fatalf("expected no running session, got %q", res.AlreadyRunningID)
	}
	if !res.OK() {
		t.Fatalf("expected OK with only done/failed sessions present")
	}
}

func TestRun_DetectsRunningSession(t *testing.T) {
	home := t.TempDir()
	writeSession(t, home, "s1", "done")
	writeSession(t, home, "s2", "running")

	c := NewWithLookPath(fakeHerdr{}, home, lookPathAllFound)
	res := c.Run()
	if res.AlreadyRunningID != "s2" {
		t.Fatalf("expected to detect running session s2, got %q", res.AlreadyRunningID)
	}
	if res.OK() {
		t.Fatalf("expected not OK while another session is running")
	}
}

func TestRun_NoSessionsDirectoryIsNotAnError(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, t.TempDir(), lookPathAllFound)
	res := c.Run()
	if res.AlreadyRunningID != "" {
		t.Fatalf("expected no running session when ~/.monday doesn't exist yet, got %q", res.AlreadyRunningID)
	}
}

func TestRun_MalformedSessionFileIsSkippedNotFatal(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".monday", "sessions", "broken")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte("not valid json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeSession(t, home, "s2", "running")

	c := NewWithLookPath(fakeHerdr{}, home, lookPathAllFound)
	res := c.Run()
	if res.AlreadyRunningID != "s2" {
		t.Fatalf("expected the malformed file skipped and s2 still detected, got %q", res.AlreadyRunningID)
	}
}
