package skillwriter

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MayurWinjit/monday/internal/domain"
)

//go:embed skill.md
var skillContent string

const dirMode = 0o755
const fileMode = 0o644

var _ domain.SkillInstaller = (*Writer)(nil)

type Writer struct {
	homeDir string
}

func New(homeDir string) *Writer {
	return &Writer{homeDir: homeDir}
}

func (w *Writer) skillDir() string {
	return filepath.Join(w.homeDir, ".claude", "skills", "monday")
}

func (w *Writer) Install() error {
	dir := w.skillDir()
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("skillwriter: ensure dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(skillContent), fileMode); err != nil {
		return fmt.Errorf("skillwriter: write %s: %w", path, err)
	}
	return nil
}
