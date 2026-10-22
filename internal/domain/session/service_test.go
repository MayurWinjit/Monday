package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

// --- fakes ---

type fakeBackend struct {
	sessions     map[string]Session
	histories    map[string]history.History
	interactions map[string]interaction.InteractionFile
	docs         map[string][]byte
	createDirErr error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		sessions:     make(map[string]Session),
		histories:    make(map[string]history.History),
		interactions: make(map[string]interaction.InteractionFile),
		docs:         make(map[string][]byte),
	}
}

func (f *fakeBackend) CreateSessionDir(id string) error { return f.createDirErr }

func (f *fakeBackend) SaveSession(s Session) error {
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeBackend) LoadSession(id string) (Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return Session{}, fmt.Errorf("fakeBackend: no session %s", id)
	}
	return s, nil
}

func (f *fakeBackend) SaveHistory(id string, h history.History) error {
	f.histories[id] = h
	return nil
}

func (f *fakeBackend) LoadHistory(id string) (history.History, error) {
	h, ok := f.histories[id]
	if !ok {
		return history.History{}, fmt.Errorf("fakeBackend: no history for %s", id)
	}
	return h, nil
}

func (f *fakeBackend) SaveInteractions(sessionID, file string, ifile interaction.InteractionFile) error {
	f.interactions[sessionID+"/"+file] = ifile
	return nil
}

func (f *fakeBackend) LoadInteractions(sessionID, file string) (interaction.InteractionFile, error) {
	ifile, ok := f.interactions[sessionID+"/"+file]
	if !ok {
		return interaction.InteractionFile{}, fmt.Errorf("fakeBackend: no interactions %s/%s", sessionID, file)
	}
	return ifile, nil
}

func (f *fakeBackend) WriteDoc(sessionID, relPath string, content []byte) error {
	f.docs[sessionID+"/"+relPath] = content
	return nil
}

func (f *fakeBackend) DocsDir(sessionID string) string { return "/fake/" + sessionID + "/docs" }

type fakeHerdr struct {
	panes          []layout.PaneInfo
	splitSeq       []string // pane IDs returned by successive SplitPane calls, in order
	splitCalls     []string // "fromPane:dir" for each SplitPane call
	startCalls     []string // "pane:agent"
	promptCalls    []string // "pane:prompt"
	sendKeysCalls  []string // "pane:keys"
	readAgentFunc  func(paneID string) (string, error)
	explainFunc    func(paneID string) (layout.AgentExplain, error)
	nextSplitIndex int
}

func (f *fakeHerdr) ListPanes(tabID string) ([]layout.PaneInfo, error) {
	return f.panes, nil
}

func (f *fakeHerdr) SplitPane(paneID string, dir layout.Direction) (string, error) {
	f.splitCalls = append(f.splitCalls, fmt.Sprintf("%s:%s", paneID, dir))
	if f.nextSplitIndex >= len(f.splitSeq) {
		return "", fmt.Errorf("fakeHerdr: no scripted split result for call %d", f.nextSplitIndex)
	}
	result := f.splitSeq[f.nextSplitIndex]
	f.nextSplitIndex++
	return result, nil
}

func (f *fakeHerdr) StartAgent(paneID, agentName string) error {
	f.startCalls = append(f.startCalls, fmt.Sprintf("%s:%s", paneID, agentName))
	return nil
}

func (f *fakeHerdr) PromptAgent(paneID, prompt string) error {
	f.promptCalls = append(f.promptCalls, fmt.Sprintf("%s:%s", paneID, prompt))
	return nil
}

func (f *fakeHerdr) SendKeys(paneID, keys string) error {
	f.sendKeysCalls = append(f.sendKeysCalls, fmt.Sprintf("%s:%s", paneID, keys))
	return nil
}

func (f *fakeHerdr) ReadAgent(paneID string) (string, error) {
	if f.readAgentFunc != nil {
		return f.readAgentFunc(paneID)
	}
	return "agent output for " + paneID, nil
}

func (f *fakeHerdr) ExplainAgent(paneID string) (layout.AgentExplain, error) {
	if f.explainFunc != nil {
		return f.explainFunc(paneID)
	}
	return layout.AgentExplain{PaneID: paneID, State: "idle"}, nil
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

// --- test helpers ---

func testPipeline(agents ...string) pipeline.Pipeline {
	return pipeline.Pipeline{Name: "test-pipeline", Agents: agents}
}

const (
	mondayPane = "w1:p1"
	tabID      = "w1:t1"
)

func newTestService(herdr *fakeHerdr, backend *fakeBackend, now time.Time, ids ...string) (*Service, *fakeBackend, *fakeHerdr) {
	if backend == nil {
		backend = newFakeBackend()
	}
	clock := fixedClock{now}
	idgen := &sequentialIDs{ids: ids}
	return NewService(backend, herdr, clock, idgen), backend, herdr
}

// --- Create ---

func TestCreate_ThreeAgentHappyPath(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	herdr := &fakeHerdr{
		panes:    []layout.PaneInfo{{PaneID: mondayPane, TabID: tabID, CWD: "/work/project"}},
		splitSeq: []string{"w1:p2" /* work area */, "w1:p3" /* right split 1 */, "w1:p4" /* right split 2 */},
	}
	svc, backend, _ := newTestService(herdr, nil, now, "session-1")

	sess, err := svc.Create(CreateSessionInput{
		Pipeline:     testPipeline("architect", "developer", "reviewer"),
		Requirement:  "build a widget",
		TabID:        tabID,
		MondayPaneID: mondayPane,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if sess.ID != "session-1" {
		t.Fatalf("expected session-1, got %s", sess.ID)
	}
	if sess.WorkingDir != "/work/project" {
		t.Fatalf("expected working dir resolved from monday pane, got %q", sess.WorkingDir)
	}
	if sess.WorkAreaPaneID != "w1:p2" {
		t.Fatalf("expected work area pane w1:p2, got %s", sess.WorkAreaPaneID)
	}
	if len(sess.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(sess.Steps))
	}

	wantPanes := []string{"w1:p2", "w1:p3", "w1:p4"}
	for i, step := range sess.Steps {
		if step.PaneID != wantPanes[i] {
			t.Fatalf("step %d: expected pane %s, got %s", i, wantPanes[i], step.PaneID)
		}
	}

	if sess.Steps[0].Status != StepRunning || sess.Steps[0].StartedAt == nil {
		t.Fatalf("expected step 0 running with StartedAt set, got %+v", sess.Steps[0])
	}
	for i := 1; i < 3; i++ {
		if sess.Steps[i].Status != StepWaiting {
			t.Fatalf("expected step %d waiting, got %s", i, sess.Steps[i].Status)
		}
	}
	if sess.Status != SessionRunning {
		t.Fatalf("expected session running, got %s", sess.Status)
	}

	// every agent started idle
	if len(herdr.startCalls) != 3 {
		t.Fatalf("expected 3 StartAgent calls, got %d: %v", len(herdr.startCalls), herdr.startCalls)
	}
	// only the first step was prompted
	if len(herdr.promptCalls) != 1 || len(herdr.sendKeysCalls) != 1 {
		t.Fatalf("expected exactly 1 prompt + 1 send-keys call, got %d/%d", len(herdr.promptCalls), len(herdr.sendKeysCalls))
	}

	// persisted
	if _, ok := backend.sessions["session-1"]; !ok {
		t.Fatalf("expected session to be persisted")
	}
	if _, ok := backend.docs["session-1/requirement.md"]; !ok {
		t.Fatalf("expected requirement.md to be written")
	}
	hist, ok := backend.histories["session-1"]
	if !ok || hist.Requirement != "build a widget" {
		t.Fatalf("expected history persisted with requirement, got %+v", hist)
	}
}

func TestCreate_DuplicateAgentGetsSuffixedInteractionsFile(t *testing.T) {
	now := time.Now()
	herdr := &fakeHerdr{
		panes:    []layout.PaneInfo{{PaneID: mondayPane, TabID: tabID, CWD: "/work"}},
		splitSeq: []string{"w1:p2", "w1:p3"},
	}
	svc, _, _ := newTestService(herdr, nil, now, "session-1")

	sess, err := svc.Create(CreateSessionInput{
		Pipeline:     testPipeline("developer", "developer"),
		Requirement:  "do the thing",
		TabID:        tabID,
		MondayPaneID: mondayPane,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.Steps[0].InteractionsFile != "developer" {
		t.Fatalf("expected first developer step file 'developer', got %s", sess.Steps[0].InteractionsFile)
	}
	if sess.Steps[1].InteractionsFile != "developer-2" {
		t.Fatalf("expected second developer step file 'developer-2', got %s", sess.Steps[1].InteractionsFile)
	}
}

func TestCreate_EmptyPipelineRejected(t *testing.T) {
	svc, _, _ := newTestService(&fakeHerdr{}, nil, time.Now(), "session-1")
	_, err := svc.Create(CreateSessionInput{
		Pipeline:     testPipeline(),
		TabID:        tabID,
		MondayPaneID: mondayPane,
	})
	if err == nil {
		t.Fatalf("expected error for empty pipeline")
	}
}

func TestCreate_UnresolvableMondayPaneFails(t *testing.T) {
	herdr := &fakeHerdr{panes: []layout.PaneInfo{{PaneID: "someone-else", TabID: tabID, CWD: "/x"}}}
	svc, _, _ := newTestService(herdr, nil, time.Now(), "session-1")

	_, err := svc.Create(CreateSessionInput{
		Pipeline:     testPipeline("architect"),
		TabID:        tabID,
		MondayPaneID: mondayPane,
	})
	if err == nil {
		t.Fatalf("expected error when monday pane is not found in tab's pane list")
	}
}

func TestResolveInteractionsFileNames(t *testing.T) {
	tests := []struct {
		agents []string
		want   []string
	}{
		{[]string{"architect", "developer", "reviewer"}, []string{"architect", "developer", "reviewer"}},
		{[]string{"developer", "developer"}, []string{"developer", "developer-2"}},
		{[]string{"developer", "developer", "developer"}, []string{"developer", "developer-2", "developer-3"}},
		{[]string{"a", "b", "a", "b", "a"}, []string{"a", "b", "a-2", "b-2", "a-3"}},
	}
	for _, tt := range tests {
		got := resolveInteractionsFileNames(tt.agents)
		if len(got) != len(tt.want) {
			t.Fatalf("resolveInteractionsFileNames(%v): expected %v, got %v", tt.agents, tt.want, got)
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("resolveInteractionsFileNames(%v): expected %v, got %v", tt.agents, tt.want, got)
			}
		}
	}
}

// --- HandleDone ---

func baseSessionForHandlers() Session {
	return Session{
		ID:         "s1",
		HerdrTabID: tabID,
		Pipeline:   testPipeline("architect", "developer", "reviewer"),
		Steps: []PipelineStep{
			{Index: 0, AgentName: "architect", InteractionsFile: "architect", PaneID: "w1:p2", Status: StepRunning},
			{Index: 1, AgentName: "developer", InteractionsFile: "developer", PaneID: "w1:p3", Status: StepWaiting},
			{Index: 2, AgentName: "reviewer", InteractionsFile: "reviewer", PaneID: "w1:p4", Status: StepWaiting},
		},
		CurrentStepIndex: 0,
		Status:           SessionRunning,
	}
}

func TestHandleDone_AdvancesToNextStep(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	backend.sessions[sess.ID] = sess
	backend.histories[sess.ID] = history.History{Requirement: "x"}

	herdr := &fakeHerdr{}
	now := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	svc, backend, _ := newTestService(herdr, backend, now)

	updated, err := svc.HandleDone(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "w1:p2", Type: notifyproto.NotifyDone})
	if err != nil {
		t.Fatalf("HandleDone: %v", err)
	}

	if updated.Steps[0].Status != StepDone || updated.Steps[0].FinishedAt == nil {
		t.Fatalf("expected step 0 done with FinishedAt, got %+v", updated.Steps[0])
	}
	if updated.Steps[1].Status != StepRunning || updated.Steps[1].StartedAt == nil {
		t.Fatalf("expected step 1 running with StartedAt, got %+v", updated.Steps[1])
	}
	if updated.CurrentStepIndex != 1 {
		t.Fatalf("expected current step index 1, got %d", updated.CurrentStepIndex)
	}
	if updated.Status != SessionRunning {
		t.Fatalf("expected session still running, got %s", updated.Status)
	}
	if len(herdr.promptCalls) != 1 || len(herdr.sendKeysCalls) != 1 {
		t.Fatalf("expected step 1 to be prompted, got prompts=%d sendkeys=%d", len(herdr.promptCalls), len(herdr.sendKeysCalls))
	}

	persisted := backend.sessions["s1"]
	if persisted.Steps[1].Status != StepRunning {
		t.Fatalf("expected persisted session to reflect the handoff, got %+v", persisted.Steps[1])
	}

	hist := backend.histories["s1"]
	if len(hist.Entries) != 1 || hist.Entries[0].AgentName != "architect" {
		t.Fatalf("expected a history entry capturing architect's tail, got %+v", hist.Entries)
	}
}

func TestHandleDone_LastStepMarksSessionDone(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	sess.Steps[0].Status = StepDone
	sess.Steps[1].Status = StepDone
	sess.Steps[2].Status = StepRunning
	sess.CurrentStepIndex = 2
	backend.sessions[sess.ID] = sess
	backend.histories[sess.ID] = history.History{Requirement: "x"}

	herdr := &fakeHerdr{}
	svc, backend, _ := newTestService(herdr, backend, time.Now())

	updated, err := svc.HandleDone(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "w1:p4", Type: notifyproto.NotifyDone})
	if err != nil {
		t.Fatalf("HandleDone: %v", err)
	}
	if updated.Status != SessionDone {
		t.Fatalf("expected session done, got %s", updated.Status)
	}
	if len(herdr.promptCalls) != 0 {
		t.Fatalf("expected no further prompts after the last step, got %v", herdr.promptCalls)
	}
	_ = backend
}

func TestHandleDone_UnknownPaneErrors(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	backend.sessions[sess.ID] = sess

	svc, _, _ := newTestService(&fakeHerdr{}, backend, time.Now())
	_, err := svc.HandleDone(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "no-such-pane", Type: notifyproto.NotifyDone})
	if err == nil {
		t.Fatalf("expected error for unknown pane")
	}
}

func TestHandleDone_ReadAgentFailureDoesNotBlockHandoff(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	backend.sessions[sess.ID] = sess
	backend.histories[sess.ID] = history.History{Requirement: "x"}

	herdr := &fakeHerdr{
		readAgentFunc: func(paneID string) (string, error) {
			return "", fmt.Errorf("pane closed")
		},
	}
	svc, backend, _ := newTestService(herdr, backend, time.Now())

	updated, err := svc.HandleDone(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "w1:p2", Type: notifyproto.NotifyDone})
	if err != nil {
		t.Fatalf("expected ReadAgent failure to be non-fatal, got %v", err)
	}
	if updated.Steps[1].Status != StepRunning {
		t.Fatalf("expected handoff to proceed despite ReadAgent failure, got %+v", updated.Steps[1])
	}
	hist := backend.histories["s1"]
	if len(hist.Entries) != 0 {
		t.Fatalf("expected no tail entry appended when ReadAgent fails, got %+v", hist.Entries)
	}
}

// --- HandleQuestion ---

func TestHandleQuestion_MarksNeedsInputAndReturnsFile(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	backend.sessions[sess.ID] = sess
	answer := "Option A"
	backend.interactions["s1/developer"] = interaction.InteractionFile{
		Agent: "developer",
		Questions: []interaction.Question{
			{ID: "q1", Question: "Which approach?", Options: []string{"Option A"}, Answer: &answer},
		},
	}
	sess.Steps[1].Status = StepRunning
	backend.sessions[sess.ID] = sess

	svc, backend, _ := newTestService(&fakeHerdr{}, backend, time.Now())

	updated, file, err := svc.HandleQuestion(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "w1:p3", Type: notifyproto.NotifyQuestion})
	if err != nil {
		t.Fatalf("HandleQuestion: %v", err)
	}
	if updated.Steps[1].Status != StepNeedsInput {
		t.Fatalf("expected step 1 needs_input, got %s", updated.Steps[1].Status)
	}
	if len(file.Questions) != 1 || file.Questions[0].ID != "q1" {
		t.Fatalf("expected the developer interactions file, got %+v", file)
	}
	if backend.sessions["s1"].Steps[1].Status != StepNeedsInput {
		t.Fatalf("expected persisted session to reflect needs_input")
	}
}

func TestHandleQuestion_UnknownPaneErrors(t *testing.T) {
	backend := newFakeBackend()
	backend.sessions["s1"] = baseSessionForHandlers()
	svc, _, _ := newTestService(&fakeHerdr{}, backend, time.Now())

	_, _, err := svc.HandleQuestion(notifyproto.NotifyMessage{SessionID: "s1", PaneID: "ghost-pane"})
	if err == nil {
		t.Fatalf("expected error for unknown pane")
	}
}

// --- PollForCrashes ---

func TestPollForCrashes_NoChangeWhenAllPanesPresent(t *testing.T) {
	backend := newFakeBackend()
	backend.sessions["s1"] = baseSessionForHandlers()
	herdr := &fakeHerdr{panes: []layout.PaneInfo{
		{PaneID: "w1:p2"}, {PaneID: "w1:p3"}, {PaneID: "w1:p4"},
	}}
	svc, _, _ := newTestService(herdr, backend, time.Now())

	updated, changed, err := svc.PollForCrashes("s1")
	if err != nil {
		t.Fatalf("PollForCrashes: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("expected no changed steps, got %v", changed)
	}
	if updated.Status != SessionRunning {
		t.Fatalf("expected session to remain running, got %s", updated.Status)
	}
}

func TestPollForCrashes_WaitingStepMissingPaneIsIgnored(t *testing.T) {
	backend := newFakeBackend()
	backend.sessions["s1"] = baseSessionForHandlers()
	// developer's pane (w1:p3) is gone, but developer is still "waiting"
	// (never started) - only running/needs_input steps are polled.
	herdr := &fakeHerdr{panes: []layout.PaneInfo{{PaneID: "w1:p2"}, {PaneID: "w1:p4"}}}
	svc, backend, _ := newTestService(herdr, backend, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))

	updated, changed, err := svc.PollForCrashes("s1")
	if err != nil {
		t.Fatalf("PollForCrashes: %v", err)
	}
	if len(changed) != 0 {
		// architect is "running" and present, developer is "waiting" (not
		// polled - it never started), so nothing should have failed here;
		// this test's base session has developer waiting, not running.
		t.Fatalf("expected no changed steps since only a 'waiting' step's pane is missing, got %v", changed)
	}
	_ = updated
	_ = backend
}

func TestPollForCrashes_RunningStepPaneGoneFailsSession(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	sess.Steps[0].Status = StepDone
	sess.Steps[1].Status = StepRunning // developer now running, pane w1:p3
	backend.sessions["s1"] = sess

	herdr := &fakeHerdr{panes: []layout.PaneInfo{{PaneID: "w1:p2"}, {PaneID: "w1:p4"}}} // w1:p3 missing
	svc, backend, _ := newTestService(herdr, backend, time.Now())

	updated, changed, err := svc.PollForCrashes("s1")
	if err != nil {
		t.Fatalf("PollForCrashes: %v", err)
	}
	if len(changed) != 1 || changed[0] != 1 {
		t.Fatalf("expected step 1 to be flagged failed, got %v", changed)
	}
	if updated.Steps[1].Status != StepFailed {
		t.Fatalf("expected step 1 failed, got %s", updated.Steps[1].Status)
	}
	if updated.Status != SessionFailed {
		t.Fatalf("expected session failed, got %s", updated.Status)
	}
	if backend.sessions["s1"].Status != SessionFailed {
		t.Fatalf("expected persisted session to be failed")
	}
}

func TestPollForCrashes_AlreadyFailedSessionIsNoOp(t *testing.T) {
	backend := newFakeBackend()
	sess := baseSessionForHandlers()
	sess.Status = SessionFailed
	backend.sessions["s1"] = sess

	herdr := &fakeHerdr{panes: nil} // every pane "missing" - must not matter, session already failed
	svc, _, _ := newTestService(herdr, backend, time.Now())

	_, changed, err := svc.PollForCrashes("s1")
	if err != nil {
		t.Fatalf("PollForCrashes: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("expected no-op for an already-failed session, got %v", changed)
	}
}
