package progress

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/graphify"
	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

// --- fakes (mirrors internal/domain/session's own test fakes) ---

type fakeSessionBackend struct {
	sessions     map[string]session.Session
	histories    map[string]history.History
	interactions map[string]interaction.InteractionFile
}

func newFakeSessionBackend() *fakeSessionBackend {
	return &fakeSessionBackend{
		sessions:     make(map[string]session.Session),
		histories:    make(map[string]history.History),
		interactions: make(map[string]interaction.InteractionFile),
	}
}

func (f *fakeSessionBackend) CreateSessionDir(id string) error { return nil }
func (f *fakeSessionBackend) SaveSession(s session.Session) error {
	f.sessions[s.ID] = s
	return nil
}
func (f *fakeSessionBackend) LoadSession(id string) (session.Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return session.Session{}, fmt.Errorf("no session %s", id)
	}
	return s, nil
}
func (f *fakeSessionBackend) SaveHistory(id string, h history.History) error {
	f.histories[id] = h
	return nil
}
func (f *fakeSessionBackend) LoadHistory(id string) (history.History, error) {
	return f.histories[id], nil
}
func (f *fakeSessionBackend) SaveInteractions(sessionID, file string, ifile interaction.InteractionFile) error {
	f.interactions[sessionID+"/"+file] = ifile
	return nil
}
func (f *fakeSessionBackend) LoadInteractions(sessionID, file string) (interaction.InteractionFile, error) {
	return f.interactions[sessionID+"/"+file], nil
}
func (f *fakeSessionBackend) WriteDoc(sessionID, relPath string, content []byte) error { return nil }
func (f *fakeSessionBackend) DocsDir(sessionID string) string                          { return "/fake/docs" }

type fakeSessionHerdr struct {
	panes []layout.PaneInfo
}

func (f *fakeSessionHerdr) ListPanes(tabID string) ([]layout.PaneInfo, error) { return f.panes, nil }
func (f *fakeSessionHerdr) SplitPane(paneID string, dir layout.Direction) (string, error) {
	return "w1:pX", nil
}
func (f *fakeSessionHerdr) StartAgent(paneID, agentName string) error { return nil }
func (f *fakeSessionHerdr) PromptAgent(paneID, prompt string) error   { return nil }
func (f *fakeSessionHerdr) SendKeys(paneID, keys string) error        { return nil }
func (f *fakeSessionHerdr) ReadAgent(paneID string) (string, error)   { return "", nil }
func (f *fakeSessionHerdr) ExplainAgent(paneID string) (layout.AgentExplain, error) {
	return layout.AgentExplain{}, nil
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

type sequentialIDs struct {
	ids  []string
	next int
}

func (s *sequentialIDs) NewID() string {
	id := s.ids[s.next]
	s.next++
	return id
}

func testServices() (*app.Services, *fakeSessionBackend) {
	backend := newFakeSessionBackend()
	herdrFake := &fakeSessionHerdr{}
	svc := session.NewService(backend, herdrFake, fixedClock{time.Now()}, &sequentialIDs{ids: []string{"s1"}})
	graphifySvc := graphify.NewService(fakeGraphBackend{}, fakeGraphHerdr{}, "/x")
	return &app.Services{Sessions: svc, Graphify: graphifySvc}, backend
}

type fakeGraphBackend struct{}

func (fakeGraphBackend) ListSavedGraphs() ([]string, error) { return nil, nil }

type fakeGraphHerdr struct{}

func (fakeGraphHerdr) PromptAgent(paneID, prompt string) error { return nil }
func (fakeGraphHerdr) SendKeys(paneID, keys string) error      { return nil }

func testSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	return sp
}

func runningSession() session.Session {
	now := time.Now()
	return session.Session{
		ID: "s1",
		Steps: []session.PipelineStep{
			{Index: 0, AgentName: "architect", PaneID: "w1:p1", Status: session.StepRunning, StartedAt: &now},
			{Index: 1, AgentName: "developer", PaneID: "w1:p2", Status: session.StepWaiting},
		},
		Status: session.SessionRunning,
	}
}

// --- tests ---

func TestView_Creating(t *testing.T) {
	m := Model{creating: true, spinner: testSpinner()}
	view := m.View(80, 24)
	if !strings.Contains(view, "Starting the pipeline") {
		t.Fatalf("expected a starting message, got %q", view)
	}
}

func TestView_CreateError(t *testing.T) {
	m := Model{createErr: "boom"}
	view := m.View(80, 24)
	if !strings.Contains(view, "boom") {
		t.Fatalf("expected the create error shown, got %q", view)
	}
}

func TestUpdate_SessionCreated_Success(t *testing.T) {
	m := Model{creating: true, spinner: testSpinner()}
	events := make(chan notifyproto.NotifyMessage, 1)
	screen, cmd := m.Update(sessionCreatedMsg{session: runningSession(), events: events})
	model := screen.(Model)
	if model.creating {
		t.Fatalf("expected creating to be false after success")
	}
	if model.session.ID != "s1" {
		t.Fatalf("expected session stored, got %+v", model.session)
	}
	if cmd == nil {
		t.Fatalf("expected a listenForNotify command")
	}
}

func TestUpdate_SessionCreated_Error(t *testing.T) {
	m := Model{creating: true, spinner: testSpinner()}
	screen, _ := m.Update(sessionCreatedMsg{err: fmt.Errorf("resolve pane: nope")})
	model := screen.(Model)
	if model.createErr == "" {
		t.Fatalf("expected createErr set")
	}
}

func TestView_StepsRendered(t *testing.T) {
	m := Model{session: runningSession(), spinner: testSpinner(), now: time.Now()}
	view := m.View(80, 24)
	if !strings.Contains(view, "architect") || !strings.Contains(view, "developer") {
		t.Fatalf("expected both steps rendered, got:\n%s", view)
	}
}

func TestUpdate_TickEveryThirdTriggersCrashPoll(t *testing.T) {
	services, _ := testServices()
	m := Model{session: runningSession(), spinner: testSpinner(), services: services}

	for i := 1; i <= 2; i++ {
		screen, cmd := m.Update(tickMsg(time.Now()))
		m = screen.(Model)
		if _, isBatch := cmd().(tea.BatchMsg); isBatch {
			t.Fatalf("tick %d: expected a plain re-tick, not a batch with a crash poll", i)
		}
	}
	// 3rd tick should batch the re-tick with a crash-poll command
	screen, cmd := m.Update(tickMsg(time.Now()))
	m = screen.(Model)
	if m.tickCount != 3 {
		t.Fatalf("expected tickCount 3, got %d", m.tickCount)
	}
	if _, isBatch := cmd().(tea.BatchMsg); !isBatch {
		t.Fatalf("expected the 3rd tick to batch a crash-poll command")
	}
}

func TestUpdate_NotifyDone_AdvancesSession(t *testing.T) {
	services, backend := testServices()
	sess := runningSession()
	backend.sessions[sess.ID] = sess
	backend.histories[sess.ID] = history.History{}

	m := Model{session: sess, services: services, spinner: testSpinner(), events: make(chan notifyproto.NotifyMessage)}
	screen, cmd := m.Update(notifyEventMsg{msg: notifyproto.NotifyMessage{Type: notifyproto.NotifyDone, SessionID: "s1", PaneID: "w1:p1"}})
	model := screen.(Model)

	if model.session.Steps[0].Status != session.StepDone {
		t.Fatalf("expected step 0 done, got %+v", model.session.Steps[0])
	}
	if model.session.Steps[1].Status != session.StepRunning {
		t.Fatalf("expected step 1 running (handed off), got %+v", model.session.Steps[1])
	}
	if cmd == nil {
		t.Fatalf("expected listenForNotify re-issued")
	}
}

func TestUpdate_NotifyQuestion_OpensQAPlaceholder(t *testing.T) {
	services, backend := testServices()
	sess := runningSession()
	backend.sessions[sess.ID] = sess
	backend.interactions["s1/developer"] = interaction.InteractionFile{
		Agent:     "developer",
		Questions: []interaction.Question{{ID: "q1", Question: "Which approach?"}},
	}

	m := Model{session: sess, services: services, spinner: testSpinner(), events: make(chan notifyproto.NotifyMessage)}
	// developer's interactions file name defaults to its agent name per
	// resolveInteractionsFileNames; set it on the step directly for the test.
	m.session.Steps[1].InteractionsFile = "developer"
	backend.sessions[sess.ID] = m.session

	screen, _ := m.Update(notifyEventMsg{msg: notifyproto.NotifyMessage{Type: notifyproto.NotifyQuestion, SessionID: "s1", PaneID: "w1:p2"}})
	model := screen.(Model)

	if model.qa == nil {
		t.Fatalf("expected the Q&A placeholder to open")
	}
	if model.session.Steps[1].Status != session.StepNeedsInput {
		t.Fatalf("expected step 1 needs_input, got %+v", model.session.Steps[1])
	}
}

func TestUpdate_AnyKeyDismissesQAPlaceholder(t *testing.T) {
	m := Model{qa: &qaPlaceholder{file: interactionFileView{Agent: "developer"}}}
	screen, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := screen.(Model)
	if model.qa != nil {
		t.Fatalf("expected the placeholder to close on any key")
	}
}

func TestUpdate_CrashCheckResult_UpdatesSession(t *testing.T) {
	sess := runningSession()
	m := Model{session: sess}
	failedSess := sess
	failedSess.Status = session.SessionFailed
	failedSess.Steps[0].Status = session.StepFailed

	screen, _ := m.Update(crashCheckResultMsg{session: failedSess, failed: []int{0}})
	model := screen.(Model)
	if model.session.Status != session.SessionFailed {
		t.Fatalf("expected session failed, got %s", model.session.Status)
	}
}

func TestView_SessionFailed(t *testing.T) {
	sess := runningSession()
	sess.Status = session.SessionFailed
	m := Model{session: sess, spinner: testSpinner(), now: time.Now()}
	view := m.View(80, 24)
	if !strings.Contains(view, "Pipeline failed") {
		t.Fatalf("expected a failure message, got %q", view)
	}
}

func TestView_SessionDone_NoGraph(t *testing.T) {
	sess := runningSession()
	sess.Status = session.SessionDone
	m := Model{session: sess, spinner: testSpinner(), now: time.Now()}
	view := m.View(80, 24)
	if !strings.Contains(view, "Pipeline complete") {
		t.Fatalf("expected completion message, got %q", view)
	}
}

func TestUpdate_SessionDone_WithGraph_YesFlow(t *testing.T) {
	services, _ := testServices()
	sess := runningSession()
	sess.Status = session.SessionDone
	sess.GraphifyGraph = "payment-portal"
	sess.GraphifySymlink = "/project/graphify-out"

	m := Model{session: sess, services: services, spinner: testSpinner(), graphLabel: "payment-portal"}

	screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model := screen.(Model)
	if !model.graphifyRequested {
		t.Fatalf("expected graphifyRequested set after 'y'")
	}
	if cmd == nil {
		t.Fatalf("expected a RequestUpdate command")
	}

	// simulate the update landing, then confirm with enter
	screen, _ = model.Update(graphifyUpdateRequestedMsg{})
	model = screen.(Model)
	screen, cleanupCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = screen.(Model)
	if cleanupCmd == nil {
		t.Fatalf("expected a cleanup command after confirming")
	}
	msg := cleanupCmd()
	if _, ok := msg.(graphifyCleanupDoneMsg); !ok {
		t.Fatalf("expected graphifyCleanupDoneMsg, got %T", msg)
	}

	screen, _ = model.Update(msg)
	model = screen.(Model)
	if !model.graphifyFinalized {
		t.Fatalf("expected graphifyFinalized set")
	}
	view := model.View(80, 24)
	if !strings.Contains(view, "Pipeline complete") {
		t.Fatalf("expected final completion message, got %q", view)
	}
}

func TestUpdate_SessionDone_WithGraph_NoFlow(t *testing.T) {
	services, _ := testServices()
	sess := runningSession()
	sess.Status = session.SessionDone
	sess.GraphifyGraph = "payment-portal"
	sess.GraphifySymlink = "/project/graphify-out"

	m := Model{session: sess, services: services, spinner: testSpinner(), graphLabel: "payment-portal"}
	screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := screen.(Model)
	if !model.graphifyAsked {
		t.Fatalf("expected graphifyAsked set after 'n'")
	}
	if cmd == nil {
		t.Fatalf("expected a cleanup command on 'n' (symlink is no longer needed once skipped)")
	}
}

func TestUpdate_QQuits(t *testing.T) {
	m := Model{session: runningSession()}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("expected 'q' to produce a quit command")
	}
}
