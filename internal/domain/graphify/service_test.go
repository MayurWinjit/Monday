package graphify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MayurWinjit/monday/internal/domain/session"
)

type fakeBackend struct {
	labels []string
	err    error
}

func (f fakeBackend) ListSavedGraphs() ([]string, error) { return f.labels, f.err }

type fakeHerdr struct {
	promptCalls   []string
	sendKeysCalls []string
	promptErr     error
	sendKeysErr   error
}

func (f *fakeHerdr) PromptAgent(paneID, prompt string) error {
	f.promptCalls = append(f.promptCalls, paneID+":"+prompt)
	return f.promptErr
}

func (f *fakeHerdr) SendKeys(paneID, keys string) error {
	f.sendKeysCalls = append(f.sendKeysCalls, paneID+":"+keys)
	return f.sendKeysErr
}

func TestListGraphs(t *testing.T) {
	graphsDir := "/home/user/.monday/graphs"
	svc := NewService(fakeBackend{labels: []string{"payment-portal", "monorepo"}}, &fakeHerdr{}, graphsDir)

	refs, err := svc.ListGraphs()
	if err != nil {
		t.Fatalf("ListGraphs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %+v", refs)
	}
	if refs[0].Label != "payment-portal" || refs[0].Path != filepath.Join(graphsDir, "payment-portal") {
		t.Fatalf("unexpected ref: %+v", refs[0])
	}
}

func TestListGraphs_PropagatesBackendError(t *testing.T) {
	svc := NewService(fakeBackend{err: os.ErrPermission}, &fakeHerdr{}, "/x")
	if _, err := svc.ListGraphs(); err == nil {
		t.Fatalf("expected an error")
	}
}

func setupGraphsDir(t *testing.T, labels ...string) string {
	t.Helper()
	home := t.TempDir()
	graphsDir := filepath.Join(home, "graphs")
	for _, label := range labels {
		if err := os.MkdirAll(filepath.Join(graphsDir, label), 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", label, err)
		}
	}
	return graphsDir
}

func TestInject_CreatesSymlink(t *testing.T) {
	graphsDir := setupGraphsDir(t, "payment-portal")
	projectRoot := t.TempDir()
	svc := NewService(fakeBackend{}, &fakeHerdr{}, graphsDir)

	symlinkPath, err := svc.Inject("payment-portal", projectRoot)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if symlinkPath != filepath.Join(projectRoot, "graphify-out") {
		t.Fatalf("unexpected symlink path: %s", symlinkPath)
	}

	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected a symlink at %s", symlinkPath)
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != filepath.Join(graphsDir, "payment-portal") {
		t.Fatalf("expected symlink to point at the saved graph, got %s", target)
	}
}

func TestInject_UnknownLabelFails(t *testing.T) {
	graphsDir := setupGraphsDir(t)
	svc := NewService(fakeBackend{}, &fakeHerdr{}, graphsDir)

	if _, err := svc.Inject("nonexistent", t.TempDir()); err == nil {
		t.Fatalf("expected an error for an unknown graph label")
	}
}

func TestInject_RefusesWhenARealDirectoryAlreadyExists(t *testing.T) {
	graphsDir := setupGraphsDir(t, "payment-portal")
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, "graphify-out"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "graphify-out", "real-file.txt"), []byte("real data"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	svc := NewService(fakeBackend{}, &fakeHerdr{}, graphsDir)
	if _, err := svc.Inject("payment-portal", projectRoot); err == nil {
		t.Fatalf("expected Inject to refuse overwriting a real directory")
	}

	// the real directory and its contents must be untouched
	data, err := os.ReadFile(filepath.Join(projectRoot, "graphify-out", "real-file.txt"))
	if err != nil {
		t.Fatalf("expected real-file.txt to survive, read error: %v", err)
	}
	if string(data) != "real data" {
		t.Fatalf("expected real-file.txt content to survive untouched")
	}
}

func TestInject_ReplacesAPreExistingSymlink(t *testing.T) {
	graphsDir := setupGraphsDir(t, "payment-portal", "monorepo")
	projectRoot := t.TempDir()
	svc := NewService(fakeBackend{}, &fakeHerdr{}, graphsDir)

	if _, err := svc.Inject("payment-portal", projectRoot); err != nil {
		t.Fatalf("first Inject: %v", err)
	}
	symlinkPath, err := svc.Inject("monorepo", projectRoot)
	if err != nil {
		t.Fatalf("second Inject (replacing stale symlink): %v", err)
	}
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != filepath.Join(graphsDir, "monorepo") {
		t.Fatalf("expected symlink to now point at monorepo, got %s", target)
	}
}

func TestRequestUpdate_PromptsLastStepPane(t *testing.T) {
	herdr := &fakeHerdr{}
	svc := NewService(fakeBackend{}, herdr, "/x")

	sess := session.Session{
		ID: "s1",
		Steps: []session.PipelineStep{
			{Index: 0, AgentName: "architect", PaneID: "w1:p1"},
			{Index: 1, AgentName: "reviewer", PaneID: "w1:p2"},
		},
	}
	if err := svc.RequestUpdate(sess); err != nil {
		t.Fatalf("RequestUpdate: %v", err)
	}
	if len(herdr.promptCalls) != 1 || herdr.promptCalls[0] != "w1:p2:/graphify update ." {
		t.Fatalf("expected a prompt to the last step's pane (w1:p2), got %v", herdr.promptCalls)
	}
	if len(herdr.sendKeysCalls) != 1 || herdr.sendKeysCalls[0] != "w1:p2:enter" {
		t.Fatalf("expected an enter keypress to the last step's pane, got %v", herdr.sendKeysCalls)
	}
}

func TestRequestUpdate_NoStepsFails(t *testing.T) {
	svc := NewService(fakeBackend{}, &fakeHerdr{}, "/x")
	if err := svc.RequestUpdate(session.Session{ID: "s1"}); err == nil {
		t.Fatalf("expected an error for a session with no steps")
	}
}

func TestCleanup_RemovesSymlink(t *testing.T) {
	graphsDir := setupGraphsDir(t, "payment-portal")
	projectRoot := t.TempDir()
	svc := NewService(fakeBackend{}, &fakeHerdr{}, graphsDir)

	symlinkPath, err := svc.Inject("payment-portal", projectRoot)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if err := svc.Cleanup(symlinkPath); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Fatalf("expected symlink to be removed, Lstat err=%v", err)
	}
}

func TestCleanup_RefusesToRemoveARealDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	realDir := filepath.Join(projectRoot, "graphify-out")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "real-file.txt"), []byte("real data"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	svc := NewService(fakeBackend{}, &fakeHerdr{}, "/x")
	if err := svc.Cleanup(realDir); err == nil {
		t.Fatalf("expected Cleanup to refuse removing a real directory")
	}
	if _, err := os.Stat(filepath.Join(realDir, "real-file.txt")); err != nil {
		t.Fatalf("expected real-file.txt to survive, got %v", err)
	}
}

func TestCleanup_MissingSymlinkIsNotAnError(t *testing.T) {
	svc := NewService(fakeBackend{}, &fakeHerdr{}, "/x")
	if err := svc.Cleanup(filepath.Join(t.TempDir(), "graphify-out")); err != nil {
		t.Fatalf("expected no error cleaning up an already-absent symlink, got %v", err)
	}
}
