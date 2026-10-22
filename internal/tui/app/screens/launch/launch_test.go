package launch

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
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

// --- fakes for session.Service, used only by the terminate-row tests
// (mirrors internal/domain/session's own test fakes, and
// progress_test.go's fakeSessionBackend/fakeSessionHerdr) ---

type fakeSessionBackend struct {
	sessions map[string]session.Session
}

func newFakeSessionBackend() *fakeSessionBackend {
	return &fakeSessionBackend{sessions: make(map[string]session.Session)}
}

func (f *fakeSessionBackend) CreateSessionDir(id string) error { return nil }
func (f *fakeSessionBackend) SaveSession(s session.Session) error {
	f.sessions[s.ID] = s
	return nil
}
func (f *fakeSessionBackend) LoadSession(id string) (session.Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, fmt.Errorf("fakeSessionBackend: no session %s", id)
	}
	return s, nil
}
func (f *fakeSessionBackend) SaveHistory(id string, h history.History) error { return nil }
func (f *fakeSessionBackend) LoadHistory(id string) (history.History, error) {
	return history.History{}, nil
}
func (f *fakeSessionBackend) SaveInteractions(sessionID, file string, ifile interaction.InteractionFile) error {
	return nil
}
func (f *fakeSessionBackend) LoadInteractions(sessionID, file string) (interaction.InteractionFile, error) {
	return interaction.InteractionFile{}, nil
}
func (f *fakeSessionBackend) WriteDoc(sessionID, relPath string, content []byte) error { return nil }
func (f *fakeSessionBackend) DocsDir(sessionID string) string                          { return "/fake/docs" }

type fakeSessionHerdr struct {
	closeCalls []string
	closeErr   error
}

func (f *fakeSessionHerdr) ListPanes(tabID string) ([]layout.PaneInfo, error) { return nil, nil }
func (f *fakeSessionHerdr) SplitPane(paneID string, dir layout.Direction) (string, error) {
	return "", nil
}
func (f *fakeSessionHerdr) StartAgent(paneID, agentName string) error { return nil }
func (f *fakeSessionHerdr) PromptAgent(paneID, prompt string) error   { return nil }
func (f *fakeSessionHerdr) SendKeys(paneID, keys string) error        { return nil }
func (f *fakeSessionHerdr) ReadAgent(paneID string) (string, error)   { return "", nil }
func (f *fakeSessionHerdr) ExplainAgent(paneID string) (layout.AgentExplain, error) {
	return layout.AgentExplain{}, nil
}
func (f *fakeSessionHerdr) ClosePane(paneID string) error {
	f.closeCalls = append(f.closeCalls, paneID)
	return f.closeErr
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

type noopIDs struct{}

func (noopIDs) NewID() string { return "" }

// testServicesWithSession wires a session.Service backed by in-memory fakes,
// pre-seeding sess under its own ID when sess.ID is non-empty, so
// Terminate(sess.ID) has something to load.
func testServicesWithSession(sess session.Session) (*app.Services, *fakeSessionHerdr) {
	backend := newFakeSessionBackend()
	if sess.ID != "" {
		backend.sessions[sess.ID] = sess
	}
	herdrFake := &fakeSessionHerdr{}
	sessions := session.NewService(backend, herdrFake, fixedClock{time.Now()}, noopIDs{})
	return &app.Services{Agents: agent.NewCatalog(fakeDiscoverer{}), Sessions: sessions}, herdrFake
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

// --- already-running / terminate row ---

func TestView_AlreadyRunning_ShowsTerminateButton(t *testing.T) {
	m, _ := Model{loading: true, alreadyRunningID: "abc-123"}.Update(pipelinesLoadedMsg{pipelines: nil})
	view := m.(Model).View(80, 24)
	if !strings.Contains(view, "abc-123") {
		t.Fatalf("expected the running session id shown, got:\n%s", view)
	}
	if !strings.Contains(view, "Terminate session") {
		t.Fatalf("expected a terminate button, got:\n%s", view)
	}
}

func TestView_NoAlreadyRunning_HidesTerminateButton(t *testing.T) {
	m, _ := Model{loading: true}.Update(pipelinesLoadedMsg{pipelines: nil})
	view := m.(Model).View(80, 24)
	if strings.Contains(view, "Terminate session") {
		t.Fatalf("expected no terminate button when nothing is running, got:\n%s", view)
	}
}

func TestUpdate_TerminateRowIsFirstAndCountsTowardRows(t *testing.T) {
	m, _ := Model{loading: true, alreadyRunningID: "abc"}.Update(pipelinesLoadedMsg{pipelines: []pipeline.Pipeline{{Name: "a"}}})
	model := m.(Model)
	if model.cursor != 0 {
		t.Fatalf("expected cursor to start on the terminate row, got %d", model.cursor)
	}

	// terminate row + 1 pipeline + create button = 3 rows; 3 downs wraps.
	for i := 0; i < 3; i++ {
		next, _ := model.Update(key("down"))
		model = next.(Model)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor to wrap back to the terminate row after 3 downs over 3 rows, got %d", model.cursor)
	}
}

func TestUpdate_EnterOnPipelineRow_StillWorksWithTerminateRowPresent(t *testing.T) {
	m, _ := Model{loading: true, services: testServices(), alreadyRunningID: "abc"}.Update(pipelinesLoadedMsg{
		pipelines: []pipeline.Pipeline{{Name: "a", Agents: []string{"architect"}}},
	})
	model := m.(Model)
	model.cursor = 1 // row 0 = terminate, row 1 = the pipeline "a"

	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a load-agents command")
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

func TestUpdate_EnterOnTerminateRow_TerminatesAndHidesRowOnSuccess(t *testing.T) {
	sess := session.Session{
		ID:             "abc",
		WorkAreaPaneID: "w1:p2",
		Status:         session.SessionRunning,
		Steps:          []session.PipelineStep{{Index: 0, PaneID: "w1:p2", Status: session.StepRunning}},
	}
	services, herdrFake := testServicesWithSession(sess)
	m, _ := Model{loading: true, services: services, alreadyRunningID: "abc"}.Update(pipelinesLoadedMsg{pipelines: nil})
	model := m.(Model)

	afterEnter, cmd := model.Update(key("enter"))
	model = afterEnter.(Model)
	if !model.terminating {
		t.Fatalf("expected the terminating flag set immediately on enter")
	}
	if cmd == nil {
		t.Fatalf("expected enter on the terminate row to produce a command")
	}

	msg := cmd()
	result, ok := msg.(terminateResultMsg)
	if !ok {
		t.Fatalf("expected terminateResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("Terminate: %v", result.err)
	}

	final, _ := model.Update(result)
	model = final.(Model)
	if !model.terminated || model.terminating {
		t.Fatalf("expected terminated=true, terminating=false after a successful result, got %+v", model)
	}
	if model.showTerminateRow() {
		t.Fatalf("expected the terminate row hidden after success")
	}
	if len(herdrFake.closeCalls) == 0 {
		t.Fatalf("expected ClosePane called for the terminated session's panes")
	}

	view := model.View(80, 24)
	if !strings.Contains(view, "terminated") {
		t.Fatalf("expected a terminated confirmation, got:\n%s", view)
	}
}

func TestUpdate_EnterOnTerminateRow_ErrorKeepsRowAndShowsMessage(t *testing.T) {
	services, _ := testServicesWithSession(session.Session{}) // "ghost" was never saved
	m, _ := Model{loading: true, services: services, alreadyRunningID: "ghost"}.Update(pipelinesLoadedMsg{pipelines: nil})
	model := m.(Model)

	afterEnter, cmd := model.Update(key("enter"))
	model = afterEnter.(Model)
	msg := cmd()
	result, ok := msg.(terminateResultMsg)
	if !ok || result.err == nil {
		t.Fatalf("expected a terminate error for an unknown session, got %+v (%T)", msg, msg)
	}

	final, _ := model.Update(result)
	model = final.(Model)
	if model.terminated || model.terminating {
		t.Fatalf("expected terminated=false, terminating=false after an error, got %+v", model)
	}
	if !model.showTerminateRow() {
		t.Fatalf("expected the terminate row to remain visible after an error")
	}

	view := model.View(80, 24)
	if !strings.Contains(view, "could not terminate") {
		t.Fatalf("expected an error message, got:\n%s", view)
	}
}
