package agent

import (
	"errors"
	"testing"
)

type fakeDiscoverer struct {
	agents []Agent
	err    error
}

func (f fakeDiscoverer) Discover(projectDir string) ([]Agent, error) {
	return f.agents, f.err
}

func TestCatalogDiscover_NoWarnings(t *testing.T) {
	c := NewCatalog(fakeDiscoverer{agents: []Agent{{Name: "architect"}}})
	agents, warnings := c.Discover("/some/project")
	if len(agents) != 1 || agents[0].Name != "architect" {
		t.Fatalf("expected 1 agent, got %+v", agents)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestCatalogDiscover_SplitsJoinedWarnings(t *testing.T) {
	joined := errors.Join(errors.New("bad file 1"), errors.New("bad file 2"))
	c := NewCatalog(fakeDiscoverer{agents: []Agent{{Name: "architect"}}, err: joined})

	agents, warnings := c.Discover("/some/project")
	if len(agents) != 1 {
		t.Fatalf("expected agents to still be returned despite warnings, got %+v", agents)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 split warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestCatalogDiscover_SingleErrorNotJoined(t *testing.T) {
	c := NewCatalog(fakeDiscoverer{err: errors.New("single failure")})
	_, warnings := c.Discover("/some/project")
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}
