package graphify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MayurWinjit/monday/internal/domain/session"
)

// Backend is the shape Service needs from the filesystem layer - just the
// ~/.monday/graphs directory listing. Defined locally (not imported from
// the root domain package) for the same import-cycle-avoidance reason as
// pipeline.Backend / session.Backend / interaction.Backend.
type Backend interface {
	ListSavedGraphs() ([]string, error)
}

// HerdrClient is the narrow slice of domain.HerdrClient Service needs to
// send the post-pipeline `/graphify update .` prompt.
type HerdrClient interface {
	PromptAgent(paneID, prompt string) error
	SendKeys(paneID, keys string) error
}

// Service is the graphify workflow use case from architect-plan section 6:
// list saved graphs, inject/remove the project-root symlink, and trigger
// the post-pipeline graph update by delegating to an already-running
// claude pane (there is no documented way for a standalone Go binary to
// invoke a Claude Code skill directly - see section 8.4).
type Service struct {
	backend   Backend
	herdr     HerdrClient
	graphsDir string // ~/.monday/graphs
}

func NewService(backend Backend, herdr HerdrClient, graphsDir string) *Service {
	return &Service{backend: backend, herdr: herdr, graphsDir: graphsDir}
}

// ListGraphs returns every saved graphify-out folder under ~/.monday/graphs,
// built from Backend's raw label list into the richer GraphRef (label +
// absolute path) so callers never have to re-derive the path themselves.
func (s *Service) ListGraphs() ([]GraphRef, error) {
	labels, err := s.backend.ListSavedGraphs()
	if err != nil {
		return nil, fmt.Errorf("graphify: list graphs: %w", err)
	}
	refs := make([]GraphRef, 0, len(labels))
	for _, label := range labels {
		refs = append(refs, GraphRef{Label: label, Path: filepath.Join(s.graphsDir, label)})
	}
	return refs, nil
}

// Inject creates <projectRoot>/graphify-out as a symlink to the saved graph
// labeled label, refusing if that path already exists and is not a symlink
// (a real folder there is left alone rather than ever overwritten - a
// conflict error is returned instead). A pre-existing symlink (e.g. left
// behind by a crashed prior run) is replaced, not treated as a conflict.
func (s *Service) Inject(label, projectRoot string) (string, error) {
	target := filepath.Join(s.graphsDir, label)
	if _, err := os.Stat(target); err != nil {
		return "", fmt.Errorf("graphify: inject: saved graph %q not found: %w", label, err)
	}

	symlinkPath := filepath.Join(projectRoot, "graphify-out")
	info, err := os.Lstat(symlinkPath)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink == 0 {
			return "", fmt.Errorf("graphify: inject: %s already exists and is not a symlink Monday manages - refusing to overwrite it", symlinkPath)
		}
		if err := os.Remove(symlinkPath); err != nil {
			return "", fmt.Errorf("graphify: inject: remove existing symlink %s: %w", symlinkPath, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// nothing there yet, proceed
	default:
		return "", fmt.Errorf("graphify: inject: stat %s: %w", symlinkPath, err)
	}

	if err := os.Symlink(target, symlinkPath); err != nil {
		return "", fmt.Errorf("graphify: inject: create symlink %s -> %s: %w", symlinkPath, target, err)
	}
	return symlinkPath, nil
}

// RequestUpdate sends `/graphify update .` into the last pipeline step's
// pane - chosen because it is the one guaranteed to already be a running
// claude process sitting in the project directory (architect-plan section
// 8.4). The user confirms manually (via the progress screen) before this
// is ever called and again confirms once the update finishes before
// Cleanup is called - this is never automatic.
func (s *Service) RequestUpdate(sess session.Session) error {
	if len(sess.Steps) == 0 {
		return fmt.Errorf("graphify: request update: session %s has no steps", sess.ID)
	}
	lastStep := sess.Steps[len(sess.Steps)-1]

	if err := s.herdr.PromptAgent(lastStep.PaneID, "/graphify update ."); err != nil {
		return fmt.Errorf("graphify: request update: prompt: %w", err)
	}
	if err := s.herdr.SendKeys(lastStep.PaneID, "enter"); err != nil {
		return fmt.Errorf("graphify: request update: send enter: %w", err)
	}
	return nil
}

// Cleanup removes the project-root symlink, verifying via os.Lstat that it
// really is a symlink before removing it - this must never delete a real
// directory. A missing symlink is treated as already clean, not an error.
func (s *Service) Cleanup(symlinkPath string) error {
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("graphify: cleanup: stat %s: %w", symlinkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("graphify: cleanup: refusing to remove %s - it is not a symlink", symlinkPath)
	}
	if err := os.Remove(symlinkPath); err != nil {
		return fmt.Errorf("graphify: cleanup: remove %s: %w", symlinkPath, err)
	}
	return nil
}
