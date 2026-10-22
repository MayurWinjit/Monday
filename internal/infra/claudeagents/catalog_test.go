package claudeagents

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

func writeAgentFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDiscover_GlobalOnly(t *testing.T) {
	home := t.TempDir()
	globalDir := filepath.Join(home, ".claude", "agents")
	writeAgentFile(t, globalDir, "architect.md", "---\nname: architect\ndescription: Plans things\n---\n\nBody text.\n")
	writeAgentFile(t, globalDir, "developer.md", "---\nname: developer\ndescription: Builds things\n---\n")

	c := New(home)
	agents, err := c.Discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	names := agentNames(agents)
	if len(names) != 2 || names["architect"] != "Plans things" || names["developer"] != "Builds things" {
		t.Fatalf("unexpected agents: %+v", agents)
	}
	for _, a := range agents {
		if a.Source != agent.AgentSourceGlobal {
			t.Fatalf("expected global source, got %+v", a)
		}
	}
}

func TestDiscover_ProjectOverridesGlobalOnNameClash(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	writeAgentFile(t, filepath.Join(home, ".claude", "agents"), "architect.md",
		"---\nname: architect\ndescription: Global architect\n---\n")
	writeAgentFile(t, filepath.Join(project, ".claude", "agents"), "architect.md",
		"---\nname: architect\ndescription: Project architect\n---\n")
	writeAgentFile(t, filepath.Join(project, ".claude", "agents"), "tester.md",
		"---\nname: tester\ndescription: Tests things\n---\n")

	c := New(home)
	agents, err := c.Discover(project)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents (architect overridden, tester added), got %+v", agents)
	}

	byName := make(map[string]agent.Agent, len(agents))
	for _, a := range agents {
		byName[a.Name] = a
	}
	arch, ok := byName["architect"]
	if !ok {
		t.Fatalf("expected architect in result: %+v", agents)
	}
	if arch.Description != "Project architect" || arch.Source != agent.AgentSourceProject {
		t.Fatalf("expected project architect to win, got %+v", arch)
	}
	tester, ok := byName["tester"]
	if !ok || tester.Source != agent.AgentSourceProject {
		t.Fatalf("expected project-only tester, got %+v", byName)
	}
}

func TestDiscover_NoProjectAgentsDirIsNotAWarning(t *testing.T) {
	home := t.TempDir()
	writeAgentFile(t, filepath.Join(home, ".claude", "agents"), "architect.md",
		"---\nname: architect\n---\n")

	c := New(home)
	agents, err := c.Discover(t.TempDir())
	if err != nil {
		t.Fatalf("expected no warnings for a missing project agents dir, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %+v", agents)
	}
}

func TestDiscover_MalformedFilesAreSkippedWithWarningsNotFatal(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "agents")

	writeAgentFile(t, dir, "good.md", "---\nname: good\ndescription: Fine\n---\n")
	writeAgentFile(t, dir, "no-frontmatter.md", "Just a regular markdown file, no frontmatter at all.\n")
	writeAgentFile(t, dir, "unterminated.md", "---\nname: unterminated\ndescription: oops\n\nBody without a closing fence.\n")
	writeAgentFile(t, dir, "empty-name.md", "---\nname: \ndescription: has no name\n---\n")
	writeAgentFile(t, dir, "not-markdown.txt", "---\nname: ignored\n---\n")

	c := New(home)
	agents, err := c.Discover("")
	if err == nil {
		t.Fatalf("expected warnings for the 3 malformed .md files")
	}
	if len(agents) != 1 || agents[0].Name != "good" {
		t.Fatalf("expected only the well-formed agent to survive, got %+v", agents)
	}

	warnings := splitForTest(err)
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings (no-frontmatter, unterminated, empty-name), got %d: %v", len(warnings), warnings)
	}
}

func agentNames(agents []agent.Agent) map[string]string {
	out := make(map[string]string, len(agents))
	for _, a := range agents {
		out[a.Name] = a.Description
	}
	return out
}

func splitForTest(err error) []error {
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		return u.Unwrap()
	}
	return []error{err}
}

func TestParseFrontmatter_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
		wantErr  bool
	}{
		{"valid", "---\nname: architect\ndescription: Plans things\n---\nBody\n", "architect", "Plans things", false},
		{"valid no description", "---\nname: architect\n---\n", "architect", "", false},
		{"missing frontmatter", "No fences here.\n", "", "", true},
		{"unterminated", "---\nname: architect\n", "", "", true},
		{"empty name", "---\nname: \n---\n", "", "", true},
		{"no name field", "---\ndescription: no name field\n---\n", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc, err := parseFrontmatter([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got name=%q desc=%q", name, desc)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName || desc != tt.wantDesc {
				t.Fatalf("expected name=%q desc=%q, got name=%q desc=%q", tt.wantName, tt.wantDesc, name, desc)
			}
		})
	}
}

func TestDiscover_ResultOrderIsStableByFilename(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "agents")
	writeAgentFile(t, dir, "z-last.md", "---\nname: zlast\n---\n")
	writeAgentFile(t, dir, "a-first.md", "---\nname: afirst\n---\n")

	c := New(home)
	agents, err := c.Discover("")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	got := []string{agents[0].Name, agents[1].Name}
	want := []string{"afirst", "zlast"}
	sort.Strings(want)
	if got[0] != "afirst" || got[1] != "zlast" {
		t.Fatalf("expected filename-sorted order, got %v", got)
	}
}
