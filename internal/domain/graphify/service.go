package graphify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MayurWinjit/monday/internal/domain/session"
)

type Backend interface {
	ListSavedGraphs() ([]string, error)
}

type HerdrClient interface {
	PromptAgent(paneID, prompt string) error
	SendKeys(paneID, keys string) error
}

type Service struct {
	backend   Backend
	herdr     HerdrClient
	graphsDir string
}

func NewService(backend Backend, herdr HerdrClient, graphsDir string) *Service {
	return &Service{backend: backend, herdr: herdr, graphsDir: graphsDir}
}

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

	default:
		return "", fmt.Errorf("graphify: inject: stat %s: %w", symlinkPath, err)
	}

	if err := os.Symlink(target, symlinkPath); err != nil {
		return "", fmt.Errorf("graphify: inject: create symlink %s -> %s: %w", symlinkPath, target, err)
	}
	return symlinkPath, nil
}

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
