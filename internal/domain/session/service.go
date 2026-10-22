package session

import (
	"fmt"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

// Service is the session lifecycle use case from architect-plan section 6:
// create a session (spawn the pane grid, start every agent idle, prompt
// only the first), advance it when a step finishes or raises questions,
// and detect a crashed/closed pane.
//
// Every method reloads the authoritative Session from Backend, mutates it,
// persists it, and returns the fresh copy - the file on disk is the single
// source of truth, and a caller holding a stale in-memory copy (the TUI's
// progress screen, or a CLI harness invoked as a separate process) simply
// replaces it with what each call returns. This is a small, deliberate
// refinement over section 6's shorthand signatures ("HandleDone(evt) error",
// "PollForCrashes(tabID) ([]int, error)") - see docs/developer-questions.md.
type Service struct {
	store Backend
	herdr HerdrClient
	clock Clock
	ids   IDGenerator
}

func NewService(store Backend, herdr HerdrClient, clock Clock, ids IDGenerator) *Service {
	return &Service{store: store, herdr: herdr, clock: clock, ids: ids}
}

// CreateSessionInput is everything Create needs beyond what it resolves
// itself (working dir, the pane grid, the new session ID).
type CreateSessionInput struct {
	Pipeline      pipeline.Pipeline
	Requirement   string
	TabID         string
	MondayPaneID  string
	GraphifyGraph string
}

// Create spawns the pipeline's pane grid, starts every agent idle, prompts
// only the first step, and persists session.json/history.json. See
// architect-plan section 5.2 for the spawn sequence and section 8.1 for the
// end-to-end walkthrough this implements.
func (s *Service) Create(input CreateSessionInput) (Session, error) {
	if len(input.Pipeline.Agents) == 0 {
		return Session{}, fmt.Errorf("session: create: pipeline has no agents")
	}

	workingDir, err := s.resolveWorkingDir(input.TabID, input.MondayPaneID)
	if err != nil {
		return Session{}, err
	}

	workPaneID, err := s.herdr.SplitPane(input.MondayPaneID, layout.DirectionRight)
	if err != nil {
		return Session{}, fmt.Errorf("session: create: split work area pane: %w", err)
	}

	logicalToPane, err := s.realizePaneGrid(workPaneID, len(input.Pipeline.Agents))
	if err != nil {
		return Session{}, err
	}

	steps := buildSteps(input.Pipeline.Agents, logicalToPane)
	for _, step := range steps {
		if err := s.herdr.StartAgent(step.PaneID, step.AgentName); err != nil {
			return Session{}, fmt.Errorf("session: create: start agent %s in pane %s: %w", step.AgentName, step.PaneID, err)
		}
	}

	id := s.ids.NewID()
	if err := s.store.CreateSessionDir(id); err != nil {
		return Session{}, fmt.Errorf("session: create: create session dir: %w", err)
	}
	if err := s.store.WriteDoc(id, "requirement.md", []byte(input.Requirement)); err != nil {
		return Session{}, fmt.Errorf("session: create: write requirement doc: %w", err)
	}

	now := s.clock.Now()
	steps[0].Status = StepRunning
	steps[0].StartedAt = &now

	sess := Session{
		ID:               id,
		PipelineName:     input.Pipeline.Name,
		Pipeline:         input.Pipeline,
		Requirement:      input.Requirement,
		HerdrTabID:       input.TabID,
		MondayPaneID:     input.MondayPaneID,
		WorkAreaPaneID:   workPaneID,
		WorkingDir:       workingDir,
		Steps:            steps,
		CurrentStepIndex: 0,
		Status:           SessionRunning,
		GraphifyGraph:    input.GraphifyGraph,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	hist := history.History{
		Requirement: input.Requirement,
		Entries: []history.HistoryEntry{
			{Timestamp: now, StepIndex: 0, Role: history.HistoryUser, Content: input.Requirement},
		},
	}
	if err := s.store.SaveHistory(id, hist); err != nil {
		return Session{}, fmt.Errorf("session: create: save history: %w", err)
	}
	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, fmt.Errorf("session: create: save session: %w", err)
	}

	if err := s.promptStep(sess, 0); err != nil {
		return Session{}, fmt.Errorf("session: create: %w", err)
	}

	return sess, nil
}

func (s *Service) resolveWorkingDir(tabID, mondayPaneID string) (string, error) {
	panes, err := s.herdr.ListPanes(tabID)
	if err != nil {
		return "", fmt.Errorf("session: resolve working dir: list panes for tab %s: %w", tabID, err)
	}
	for _, p := range panes {
		if p.PaneID == mondayPaneID {
			return p.CWD, nil
		}
	}
	return "", fmt.Errorf("session: resolve working dir: monday pane %s not found in tab %s", mondayPaneID, tabID)
}

// realizePaneGrid walks a layout.Plan, resolving each logical pane index to
// a real herdr pane ID, seeded with workPaneID at logical index 0.
func (s *Service) realizePaneGrid(workPaneID string, n int) (map[int]string, error) {
	logicalToPane := map[int]string{0: workPaneID}
	for _, instr := range layout.Plan(n) {
		fromPane, ok := logicalToPane[instr.FromLogical]
		if !ok {
			return nil, fmt.Errorf("session: create: layout plan referenced unresolved logical pane %d", instr.FromLogical)
		}
		newPane, err := s.herdr.SplitPane(fromPane, instr.Direction)
		if err != nil {
			return nil, fmt.Errorf("session: create: split pane for logical index %d: %w", instr.NewLogical, err)
		}
		logicalToPane[instr.NewLogical] = newPane
	}
	return logicalToPane, nil
}

// buildSteps assembles the initial Steps slice, all StepWaiting, resolving
// each step's pane from logicalToPane and its interactions file name via
// resolveInteractionsFileNames (handles the same agent appearing more than
// once in one pipeline - see docs/developer-questions.md / challenge #5).
func buildSteps(agents []string, logicalToPane map[int]string) []PipelineStep {
	names := resolveInteractionsFileNames(agents)
	steps := make([]PipelineStep, len(agents))
	for i, agentName := range agents {
		steps[i] = PipelineStep{
			Index:            i,
			AgentName:        agentName,
			InteractionsFile: names[i],
			PaneID:           logicalToPane[i],
			Status:           StepWaiting,
		}
	}
	return steps
}

func resolveInteractionsFileNames(agents []string) []string {
	counts := make(map[string]int, len(agents))
	names := make([]string, len(agents))
	for i, a := range agents {
		counts[a]++
		if counts[a] == 1 {
			names[i] = a
		} else {
			names[i] = fmt.Sprintf("%s-%d", a, counts[a])
		}
	}
	return names
}

// promptStep sends the fixed-template handoff prompt (architect-plan
// section 5.2) to the given step's pane, followed by an enter keypress to
// submit it.
func (s *Service) promptStep(sess Session, stepIndex int) error {
	step := sess.Steps[stepIndex]
	if err := s.herdr.PromptAgent(step.PaneID, buildPrompt(sess, stepIndex)); err != nil {
		return fmt.Errorf("prompt step %d (%s): %w", stepIndex, step.AgentName, err)
	}
	if err := s.herdr.SendKeys(step.PaneID, "enter"); err != nil {
		return fmt.Errorf("send enter to step %d (%s): %w", stepIndex, step.AgentName, err)
	}
	return nil
}

func buildPrompt(sess Session, stepIndex int) string {
	step := sess.Steps[stepIndex]
	return fmt.Sprintf(
		"You are running as part of a Monday pipeline. "+
			"SESSION_ID=%s PANE_ID=%s AGENT_NAME=%s STEP=%d/%d INTERACTIONS_FILE=%s.json. "+
			"Read ~/.monday/sessions/%s/docs/requirement.md for the task, "+
			"~/.monday/sessions/%s/history.json for prior steps, and "+
			"~/.monday/sessions/%s/docs/ for any generated files. "+
			"When you need user input, write your questions to "+
			"~/.monday/sessions/%s/interactions/%s.json (see the monday skill for the exact format) "+
			"and then run: monday notify --session %s --pane %s --type QUES. "+
			"When you are completely done with your part, run: monday notify --session %s --pane %s --type DONE",
		sess.ID, step.PaneID, step.AgentName, stepIndex+1, len(sess.Steps), step.InteractionsFile,
		sess.ID,
		sess.ID,
		sess.ID,
		sess.ID, step.InteractionsFile,
		sess.ID, step.PaneID,
		sess.ID, step.PaneID,
	)
}

func findStepByPane(sess Session, paneID string) (int, bool) {
	for i, step := range sess.Steps {
		if step.PaneID == paneID {
			return i, true
		}
	}
	return 0, false
}

// HandleDone marks the step at msg.PaneID done, captures a best-effort tail
// of its terminal output into history, and either hands off to the next
// step (prompting it and flipping it to running) or marks the whole session
// done if that was the last step.
func (s *Service) HandleDone(msg notifyproto.NotifyMessage) (Session, error) {
	sess, err := s.store.LoadSession(msg.SessionID)
	if err != nil {
		return Session{}, fmt.Errorf("session: handle done: load session %s: %w", msg.SessionID, err)
	}

	idx, ok := findStepByPane(sess, msg.PaneID)
	if !ok {
		return Session{}, fmt.Errorf("session: handle done: no step in session %s matches pane %s", msg.SessionID, msg.PaneID)
	}

	now := s.clock.Now()
	sess.Steps[idx].Status = StepDone
	sess.Steps[idx].FinishedAt = &now
	sess.UpdatedAt = now

	if err := s.appendHistoryTail(sess, idx, now); err != nil {
		return Session{}, fmt.Errorf("session: handle done: append history: %w", err)
	}

	nextIdx := idx + 1
	if nextIdx < len(sess.Steps) {
		sess.Steps[nextIdx].Status = StepRunning
		sess.Steps[nextIdx].StartedAt = &now
		sess.CurrentStepIndex = nextIdx

		if err := s.promptStep(sess, nextIdx); err != nil {
			return Session{}, fmt.Errorf("session: handle done: %w", err)
		}
	} else {
		sess.Status = SessionDone
	}

	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, fmt.Errorf("session: handle done: save session: %w", err)
	}
	return sess, nil
}

// appendHistoryTail does a best-effort ReadAgent capture of the finishing
// step's terminal output into history.json: a ReadAgent failure simply
// means no tail entry is appended for this step-completion event (the
// handoff must never block on it), but an actual history.json persistence
// failure is a real error and is returned.
func (s *Service) appendHistoryTail(sess Session, stepIdx int, now time.Time) error {
	hist, err := s.store.LoadHistory(sess.ID)
	if err != nil {
		hist = history.History{Requirement: sess.Requirement}
	}

	if tail, readErr := s.herdr.ReadAgent(sess.Steps[stepIdx].PaneID); readErr == nil {
		hist.Entries = append(hist.Entries, history.HistoryEntry{
			Timestamp: now,
			StepIndex: stepIdx,
			AgentName: sess.Steps[stepIdx].AgentName,
			Role:      history.HistoryAgent,
			Content:   tail,
		})
	}

	return s.store.SaveHistory(sess.ID, hist)
}

// HandleQuestion marks the step at msg.PaneID as needing input and returns
// its pending interactions file for the TUI to render.
func (s *Service) HandleQuestion(msg notifyproto.NotifyMessage) (Session, interaction.InteractionFile, error) {
	sess, err := s.store.LoadSession(msg.SessionID)
	if err != nil {
		return Session{}, interaction.InteractionFile{}, fmt.Errorf("session: handle question: load session %s: %w", msg.SessionID, err)
	}

	idx, ok := findStepByPane(sess, msg.PaneID)
	if !ok {
		return Session{}, interaction.InteractionFile{}, fmt.Errorf("session: handle question: no step in session %s matches pane %s", msg.SessionID, msg.PaneID)
	}

	sess.Steps[idx].Status = StepNeedsInput
	sess.UpdatedAt = s.clock.Now()

	file, err := s.store.LoadInteractions(sess.ID, sess.Steps[idx].InteractionsFile)
	if err != nil {
		return Session{}, interaction.InteractionFile{}, fmt.Errorf("session: handle question: load interactions: %w", err)
	}

	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, interaction.InteractionFile{}, fmt.Errorf("session: handle question: save session: %w", err)
	}
	return sess, file, nil
}

// PollForCrashes compares every step currently running or needing input
// against a live herdr pane list; any step whose pane has disappeared is
// flipped to failed, the whole session is marked failed (per analyst
// resolution #11 - no further steps are activated past a crash), and the
// changed step indices are returned so the caller can highlight them.
func (s *Service) PollForCrashes(sessionID string) (Session, []int, error) {
	sess, err := s.store.LoadSession(sessionID)
	if err != nil {
		return Session{}, nil, fmt.Errorf("session: poll for crashes: load session %s: %w", sessionID, err)
	}
	if sess.Status != SessionRunning {
		return sess, nil, nil
	}

	panes, err := s.herdr.ListPanes(sess.HerdrTabID)
	if err != nil {
		return Session{}, nil, fmt.Errorf("session: poll for crashes: list panes: %w", err)
	}
	present := make(map[string]bool, len(panes))
	for _, p := range panes {
		present[p.PaneID] = true
	}

	now := s.clock.Now()
	var changed []int
	for i := range sess.Steps {
		step := &sess.Steps[i]
		if step.Status != StepRunning && step.Status != StepNeedsInput {
			continue
		}
		if !present[step.PaneID] {
			step.Status = StepFailed
			step.FinishedAt = &now
			changed = append(changed, i)
		}
	}

	if len(changed) == 0 {
		return sess, nil, nil
	}

	sess.Status = SessionFailed
	sess.UpdatedAt = now
	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, nil, fmt.Errorf("session: poll for crashes: save session: %w", err)
	}
	return sess, changed, nil
}
