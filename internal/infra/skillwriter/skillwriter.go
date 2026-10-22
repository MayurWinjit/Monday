// Package skillwriter installs the `monday` Claude Code skill at
// ~/.claude/skills/monday/SKILL.md (architect-plan section 9). The content
// is embedded from skill.md (checked into the repo, editable without
// touching Go code) and written unmodified - session-specific values
// (SESSION_ID/PANE_ID/AGENT_NAME) are supplied per-run in the prompt text
// (section 5.2), never baked into the installed file itself.
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

// Writer implements domain.SkillInstaller rooted at <homeDir>/.claude.
// homeDir is injected rather than resolved via os.UserHomeDir so tests can
// point it at t.TempDir().
type Writer struct {
	homeDir string
}

func New(homeDir string) *Writer {
	return &Writer{homeDir: homeDir}
}

func (w *Writer) skillDir() string {
	return filepath.Join(w.homeDir, ".claude", "skills", "monday")
}

// Install writes SKILL.md, creating the directory if needed. It is
// idempotent - the file is small, static, and versioned with the binary,
// so an unconditional overwrite every launch is simpler and safer than
// diffing (architect-plan section 5.5).
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
