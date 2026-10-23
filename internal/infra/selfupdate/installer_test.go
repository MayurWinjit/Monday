package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "monday")

	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := ReplaceFile(path, []byte("new")); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("expected the replaced file to be executable, got mode %v", info.Mode())
	}
}

func TestReplaceFile_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "monday")

	if err := ReplaceFile(path, []byte("fresh")); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "fresh" {
		t.Fatalf("content = %q, want %q", got, "fresh")
	}
}

func TestReplaceFile_NoLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "monday")

	if err := ReplaceFile(path, []byte("v1")); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one file in %s, got %d", dir, len(entries))
	}
}
