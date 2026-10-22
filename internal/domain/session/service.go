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

type Service struct {
	store Backend
	herdr HerdrClient
	clock Clock
	ids   IDGenerator
}

func NewService(store Backend, herdr HerdrClient, clock Clock, ids IDGenerator) *Service {
	return &Service{store: store, herdr: herdr, clock: clock, ids: ids}
}

type CreateSessionInput struct {
	Pipeline      pipeline.Pipeline
	Requirement   string
	TabID         string
	MondayPaneID  string
	GraphifyGraph string
}

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

func (s *Service) ResumeStep(sessionID, paneID, resumePrompt string) (Session, error) {
	sess, err := s.store.LoadSession(sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("session: resume step: load session %s: %w", sessionID, err)
	}

	idx, ok := findStepByPane(sess, paneID)
	if !ok {
		return Session{}, fmt.Errorf("session: resume step: no step in session %s matches pane %s", sessionID, paneID)
	}
	if sess.Steps[idx].Status != StepNeedsInput {
		return Session{}, fmt.Errorf("session: resume step: step %d (%s) is not needs_input (status=%s)", idx, sess.Steps[idx].AgentName, sess.Steps[idx].Status)
	}

	sess.Steps[idx].Status = StepRunning
	sess.UpdatedAt = s.clock.Now()

	if err := s.herdr.PromptAgent(paneID, resumePrompt); err != nil {
		return Session{}, fmt.Errorf("session: resume step: prompt: %w", err)
	}
	if err := s.herdr.SendKeys(paneID, "enter"); err != nil {
		return Session{}, fmt.Errorf("session: resume step: send enter: %w", err)
	}

	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, fmt.Errorf("session: resume step: save session: %w", err)
	}
	return sess, nil
}

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

func (s *Service) Terminate(sessionID string) (Session, error) {
	sess, err := s.store.LoadSession(sessionID)
	if err != nil {
		return Session{}, fmt.Errorf("session: terminate: load session %s: %w", sessionID, err)
	}

	now := s.clock.Now()
	closed := make(map[string]bool, len(sess.Steps)+1)
	closePane := func(paneID string) {
		if paneID == "" || closed[paneID] {
			return
		}
		closed[paneID] = true
		_ = s.herdr.ClosePane(paneID)
	}

	closePane(sess.WorkAreaPaneID)
	for i := range sess.Steps {
		step := &sess.Steps[i]
		closePane(step.PaneID)
		if step.Status == StepRunning || step.Status == StepNeedsInput || step.Status == StepWaiting {
			step.Status = StepFailed
			step.FinishedAt = &now
		}
	}

	sess.Status = SessionFailed
	sess.UpdatedAt = now

	if err := s.store.SaveSession(sess); err != nil {
		return Session{}, fmt.Errorf("session: terminate: save session: %w", err)
	}
	return sess, nil
}
