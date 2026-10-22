package skillwriter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_WritesSkillFile(t *testing.T) {
	home := t.TempDir()
	w := New(home)

	if err := w.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}

	path := filepath.Join(home, ".claude", "skills", "monday", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected SKILL.md to exist: %v", err)
	}

	content := string(data)
	for _, want := range []string{
		"name: monday",
		"monday notify --session",
		"interactions/<AGENT_NAME>.json",
		"graphify query",
		"graphify-out",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected SKILL.md to contain %q, got:\n%s", want, content)
		}
	}
}

func TestInstall_CreatesParentDirectories(t *testing.T) {
	home := t.TempDir()
	w := New(home)

	if err := w.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dir := filepath.Join(home, ".claude", "skills", "monday")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected skill dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", dir)
	}
}

func TestInstall_IsIdempotent(t *testing.T) {
	home := t.TempDir()
	w := New(home)

	if err := w.Install(); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	if err := w.Install(); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	path := filepath.Join(home, ".claude", "skills", "monday", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second install: %v", err)
	}
	if string(data) != skillContent {
		t.Fatalf("expected the file content to exactly match the embedded skill.md")
	}
}

func TestInstall_OverwritesExistingFile(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "skills", "monday")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("stale content from a prior version"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	w := New(home)
	if err := w.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "stale content") {
		t.Fatalf("expected stale content to be overwritten")
	}
}
