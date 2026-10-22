// Package claudeagents discovers Claude Code agent definitions from
// ~/.claude/agents/*.md (global) and <projectDir>/.claude/agents/*.md
// (project), merging them per architect-plan section 3.1 (project wins on a
// name clash). A malformed agent file is skipped with a warning rather than
// failing discovery for the whole catalog - see architect-plan section 5.4.
package claudeagents

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/MayurWinjit/monday/internal/domain"
	"github.com/MayurWinjit/monday/internal/domain/agent"
)

var (
	_ domain.AgentCatalog = (*Catalog)(nil)
	_ agent.Discoverer    = (*Catalog)(nil)
)

// Catalog implements domain.AgentCatalog (and the narrower agent.Discoverer
// the domain-level agent.Catalog wraps). homeDir is injected rather than
// resolved via os.UserHomeDir so tests can point it at t.TempDir().
type Catalog struct {
	homeDir string
}

func New(homeDir string) *Catalog {
	return &Catalog{homeDir: homeDir}
}

// Discover reads the global and (when projectDir is non-empty and its
// .claude/agents directory exists) project agent directories, merges them,
// and returns the merged list plus every skipped-file warning joined into a
// single error via errors.Join. A non-nil error here is always non-fatal -
// see agent.Catalog.Discover, which unpacks it back into a slice.
func (c *Catalog) Discover(projectDir string) ([]agent.Agent, error) {
	global, globalWarnings := discoverDir(filepath.Join(c.homeDir, ".claude", "agents"), agent.AgentSourceGlobal)

	var project []agent.Agent
	var projectWarnings []error
	if projectDir != "" {
		project, projectWarnings = discoverDir(filepath.Join(projectDir, ".claude", "agents"), agent.AgentSourceProject)
	}

	merged := agent.Merge(global, project)
	warnings := append(globalWarnings, projectWarnings...)
	return merged, errors.Join(warnings...)
}

// discoverDir parses every *.md file directly inside dir as an agent
// definition. A missing directory is not a warning (a project with no
// .claude/agents folder is normal); a file with no frontmatter, an
// unterminated frontmatter block, or an empty name is skipped with one
// warning each.
func discoverDir(dir string, source agent.AgentSource) ([]agent.Agent, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("claudeagents: read %s: %w", dir, err)}
	}

	agents := make([]agent.Agent, 0, len(entries))
	var warnings []error
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("claudeagents: read %s: %w", path, err))
			continue
		}
		name, description, err := parseFrontmatter(data)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("claudeagents: %s: %w", path, err))
			continue
		}
		agents = append(agents, agent.Agent{
			Name:        name,
			Description: description,
			Source:      source,
			FilePath:    path,
		})
	}
	return agents, warnings
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseFrontmatter extracts the "---\n...\n---" YAML block at the top of an
// agent markdown file.
func parseFrontmatter(data []byte) (name, description string, err error) {
	content := string(data)
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("missing frontmatter block")
	}
	rest := content[len("---\n"):]

	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", fmt.Errorf("unterminated frontmatter block")
	}
	block := rest[:end]

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(block), &fm); err != nil {
		return "", "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" {
		return "", "", fmt.Errorf("frontmatter missing name")
	}
	return fm.Name, fm.Description, nil
}
