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

type Catalog struct {
	homeDir string
}

func New(homeDir string) *Catalog {
	return &Catalog{homeDir: homeDir}
}

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
