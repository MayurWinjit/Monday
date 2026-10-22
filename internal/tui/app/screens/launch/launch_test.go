package launch

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

type fakeDiscoverer struct {
	agents []agent.Agent
}

func (f fakeDiscoverer) Discover(projectDir string) ([]agent.Agent, error) {
	return f.agents, nil
}

func testServices(agents ...agent.Agent) *app.Services {
	return &app.Services{Agents: agent.NewCatalog(fakeDiscoverer{agents: agents})}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestView_LoadingState(t *testing.T) {
	m := Model{loading: true}
	view := m.View(80, 24)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("expected a loading message, got %q", view)
	}
}

func TestView_NoSavedPipelines_HidesSection(t *testing.T) {
	m, _ := Model{loading: true}.Update(pipelinesLoadedMsg{pipelines: nil})
	view := m.View(80, 24)
	if strings.Contains(view, "Saved pipelines") {
		t.Fatalf("expected the saved pipelines section hidden when there are none, got:\n%s", view)
	}
	if !strings.Contains(view, "Create an orchestration") {
		t.Fatalf("expected the create button always present, got:\n%s", view)
	}
}

func TestView_WithSavedPipelines_ShowsSectionAndButton(t *testing.T) {
	m, _ := Model{loading: true}.Update(pipelinesLoadedMsg{pipelines: []pipeline.Pipeline{
		{Name: "trio", Agents: []string{"architect", "developer", "reviewer"}, UpdatedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
	}})
	view := m.View(80, 24)
	if !strings.Contains(view, "Saved pipelines") || !strings.Contains(view, "trio") || !strings.Contains(view, "3 agents") {
		t.Fatalf("expected saved pipeline listed with agent count, got:\n%s", view)
	}
	if !strings.Contains(view, "Create an orchestration") {
		t.Fatalf("expected the create button still present, got:\n%s", view)
	}
}

func TestUpdate_CursorWrapsAcrossPipelinesAndButton(t *testing.T) {
	m, _ := Model{loading: true}.Update(pipelinesLoadedMsg{pipelines: []pipeline.Pipeline{{Name: "a"}, {Name: "b"}}})
	model := m.(Model)
	if model.cursor != 0 {
		t.Fatalf("expected cursor to start at 0")
	}

	// down, down, down -> wraps back to 0 (2 pipelines + 1 button = 3 rows)
	for i := 0; i < 3; i++ {
		next, _ := model.Update(key("down"))
		model = next.(Model)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor to wrap back to 0 after 3 downs over 3 rows, got %d", model.cursor)
	}

	prev, _ := model.Update(key("up"))
	model = prev.(Model)
	if model.cursor != 2 {
		t.Fatalf("expected up from 0 to wrap to the last row (2), got %d", model.cursor)
	}
}

func TestUpdate_EnterOnSavedPipeline_TriggersAgentLoad(t *testing.T) {
	m, _ := Model{loading: true, services: testServices()}.Update(pipelinesLoadedMsg{pipelines: []pipeline.Pipeline{{Name: "a", Agents: []string{"architect"}}}})
	model := m.(Model) // cursor 0 = the saved pipeline row

	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected enter to produce a load-agents command")
	}
	msg := cmd()
	result, ok := msg.(agentsLoadedMsg)
	if !ok {
		t.Fatalf("expected agentsLoadedMsg, got %T", msg)
	}
	if result.forPipeline == nil || result.forPipeline.Name != "a" {
		t.Fatalf("expected forPipeline set to the selected pipeline, got %+v", result.forPipeline)
	}
}

func TestUpdate_EnterOnCreateButton_NoForPipeline(t *testing.T) {
	m, _ := Model{loading: true, services: testServices()}.Update(pipelinesLoadedMsg{pipelines: nil})
	model := m.(Model) // only row is "Create an orchestration"

	_, cmd := model.Update(key("enter"))
	msg := cmd()
	result, ok := msg.(agentsLoadedMsg)
	if !ok {
		t.Fatalf("expected agentsLoadedMsg, got %T", msg)
	}
	if result.forPipeline != nil {
		t.Fatalf("expected forPipeline nil for the create-new path, got %+v", result.forPipeline)
	}
}

func TestView_ErrorState(t *testing.T) {
	m, _ := Model{loading: true}.Update(pipelinesLoadedMsg{err: errBoom{}})
	view := m.View(80, 24)
	if !strings.Contains(view, "Could not load saved pipelines") {
		t.Fatalf("expected an error message, got %q", view)
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestUpdate_AgentsLoaded_UnknownAgentShowsBannerNotSwitch(t *testing.T) {
	m := Model{services: testServices()} // catalog has zero agents
	p := pipeline.Pipeline{Name: "trio", Agents: []string{"ghostwriter"}}

	_, cmd := m.Update(agentsLoadedMsg{agents: nil, forPipeline: &p})
	if cmd == nil {
		t.Fatalf("expected a banner command for an unresolvable agent")
	}
	msg := cmd()
	if _, ok := msg.(tea.Msg); !ok {
		t.Fatalf("expected a tea.Msg from the banner command")
	}
}

func TestUpdate_AgentsLoaded_ValidPipelineSwitchesToRequirement(t *testing.T) {
	m := Model{services: testServices(agent.Agent{Name: "architect"})}
	p := pipeline.Pipeline{Name: "solo", Agents: []string{"architect"}}

	_, cmd := m.Update(agentsLoadedMsg{agents: []agent.Agent{{Name: "architect"}}, forPipeline: &p})
	if cmd == nil {
		t.Fatalf("expected a switch-screen command")
	}
}
