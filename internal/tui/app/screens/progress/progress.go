package progress

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
	"github.com/MayurWinjit/monday/internal/infra/socket"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/qa"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	services *app.Services

	pipeline    pipeline.Pipeline
	requirement string
	graphLabel  string

	creating  bool
	createErr string

	session   session.Session
	events    <-chan notifyproto.NotifyMessage
	now       time.Time
	tickCount int
	spinner   spinner.Model
	handleErr string

	qa *qa.Model

	graphifyAsked     bool
	graphifyRequested bool
	graphifyFinalized bool
}

func New(services *app.Services, payload tuiapp.ProgressPayload) Model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	return Model{
		services:    services,
		pipeline:    payload.Pipeline,
		requirement: payload.Requirement,
		graphLabel:  payload.Graph,
		creating:    true,
		spinner:     sp,
		now:         time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		createSessionCmd(m.services, m.pipeline, m.requirement, m.graphLabel),
		m.spinner.Tick,
		tickCmd(),
	)
}

type sessionCreatedMsg struct {
	session session.Session
	events  <-chan notifyproto.NotifyMessage
	err     error
}

func createSessionCmd(services *app.Services, p pipeline.Pipeline, requirement, graphLabel string) tea.Cmd {
	return func() tea.Msg {
		tabID, err := services.Herdr.CurrentTabID()
		if err != nil {
			return sessionCreatedMsg{err: fmt.Errorf("resolve current tab: %w", err)}
		}
		paneID, err := services.Herdr.CurrentPaneID()
		if err != nil {
			return sessionCreatedMsg{err: fmt.Errorf("resolve current pane: %w", err)}
		}

		sess, err := services.Sessions.Create(session.CreateSessionInput{
			Pipeline:      p,
			Requirement:   requirement,
			TabID:         tabID,
			MondayPaneID:  paneID,
			GraphifyGraph: graphLabel,
		})
		if err != nil {
			return sessionCreatedMsg{err: fmt.Errorf("create session: %w", err)}
		}

		if graphLabel != "" {
			if symlinkPath, injErr := services.Graphify.Inject(graphLabel, sess.WorkingDir); injErr == nil {
				sess.GraphifySymlink = symlinkPath
				_ = services.Store.SaveSession(sess)
			}

		}

		server := socket.NewServer(services.Store.Root())
		events, _, err := server.Listen(sess.ID)
		if err != nil {
			return sessionCreatedMsg{session: sess, err: fmt.Errorf("listen for notify: %w", err)}
		}
		return sessionCreatedMsg{session: sess, events: events}
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type notifyEventMsg struct {
	msg notifyproto.NotifyMessage
}

func listenForNotify(events <-chan notifyproto.NotifyMessage) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-events
		if !ok {
			return nil
		}
		return notifyEventMsg{msg: evt}
	}
}

type crashCheckResultMsg struct {
	session session.Session
	failed  []int
	err     error
}

func pollForCrashesCmd(services *app.Services, sessionID string) tea.Cmd {
	return func() tea.Msg {
		sess, failed, err := services.Sessions.PollForCrashes(sessionID)
		return crashCheckResultMsg{session: sess, failed: failed, err: err}
	}
}

type graphifyUpdateRequestedMsg struct{ err error }

func requestGraphifyUpdateCmd(services *app.Services, sess session.Session) tea.Cmd {
	return func() tea.Msg {
		return graphifyUpdateRequestedMsg{err: services.Graphify.RequestUpdate(sess)}
	}
}

type graphifyCleanupDoneMsg struct{ err error }

func graphifyCleanupCmd(services *app.Services, symlinkPath string) tea.Cmd {
	return func() tea.Msg {
		return graphifyCleanupDoneMsg{err: services.Graphify.Cleanup(symlinkPath)}
	}
}

type qaOpenedMsg struct {
	file interaction.InteractionFile
	step session.PipelineStep
	err  error
}

func openQACmd(services *app.Services, sessionID string, step session.PipelineStep) tea.Cmd {
	return func() tea.Msg {
		file, err := services.Interactions.Load(sessionID, step.InteractionsFile)
		return qaOpenedMsg{file: file, step: step, err: err}
	}
}

func (m Model) currentNeedsInputStep() (session.PipelineStep, bool) {
	for _, step := range m.session.Steps {
		if step.Status == session.StepNeedsInput {
			return step, true
		}
	}
	return session.PipelineStep{}, false
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.createErr = msg.err.Error()
			return m, nil
		}
		m.session = msg.session
		m.events = msg.events
		return m, listenForNotify(m.events)

	case tickMsg:
		m.now = time.Time(msg)
		m.tickCount++
		if m.creating || m.createErr != "" || m.session.Status != session.SessionRunning {
			return m, tickCmd()
		}
		if m.tickCount%3 == 0 {
			return m, tea.Batch(tickCmd(), pollForCrashesCmd(m.services, m.session.ID))
		}
		return m, tickCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case notifyEventMsg:
		return m.handleNotify(msg.msg)

	case crashCheckResultMsg:
		if msg.err == nil {
			m.session = msg.session
		}
		return m, nil

	case qaOpenedMsg:
		if msg.err != nil {
			m.handleErr = msg.err.Error()
			return m, nil
		}
		opened := qa.New(m.services.Interactions, m.services.Sessions, m.session.ID, msg.step.PaneID, msg.step.AgentName, msg.step.InteractionsFile, msg.file)
		m.qa = &opened
		return m, nil

	case qa.SubmitResultMsg:

		if m.qa != nil {
			updated, cmd := m.qa.Update(msg)
			m.qa = &updated
			return m, cmd
		}
		return m, nil

	case qa.SubmittedMsg:
		m.qa = nil
		if msg.Err != nil {
			m.handleErr = msg.Err.Error()
			return m, nil
		}
		m.session = msg.Session
		return m, nil

	case qa.ClosedMsg:
		m.qa = nil
		return m, nil

	case graphifyUpdateRequestedMsg:
		if msg.err != nil {
			m.handleErr = msg.err.Error()
			m.graphifyFinalized = true
		}
		return m, nil

	case graphifyCleanupDoneMsg:
		m.graphifyFinalized = true
		if msg.err != nil {
			m.handleErr = msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleNotify(evt notifyproto.NotifyMessage) (tuiapp.Screen, tea.Cmd) {
	switch evt.Type {
	case notifyproto.NotifyDone:
		sess, err := m.services.Sessions.HandleDone(evt)
		if err != nil {
			m.handleErr = err.Error()
			return m, listenForNotify(m.events)
		}
		m.session = sess
		return m, listenForNotify(m.events)

	case notifyproto.NotifyQuestion:
		sess, file, err := m.services.Sessions.HandleQuestion(evt)
		if err != nil {
			m.handleErr = err.Error()
			return m, listenForNotify(m.events)
		}
		m.session = sess
		step, ok := m.currentNeedsInputStep()
		if ok {
			opened := qa.New(m.services.Interactions, m.services.Sessions, m.session.ID, step.PaneID, step.AgentName, step.InteractionsFile, file)
			m.qa = &opened
		}
		return m, listenForNotify(m.events)

	default:
		return m, listenForNotify(m.events)
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tuiapp.Screen, tea.Cmd) {
	if m.qa != nil {
		updated, cmd := m.qa.Update(msg)
		m.qa = &updated
		return m, cmd
	}

	if m.session.Status == session.SessionDone {
		switch msg.String() {
		case "y":
			if m.graphLabel != "" && !m.graphifyAsked {
				m.graphifyAsked = true
				m.graphifyRequested = true
				return m, requestGraphifyUpdateCmd(m.services, m.session)
			}
		case "n":
			if m.graphLabel != "" && !m.graphifyAsked {
				m.graphifyAsked = true
				if m.session.GraphifySymlink != "" {
					return m, graphifyCleanupCmd(m.services, m.session.GraphifySymlink)
				}
				m.graphifyFinalized = true
			}
		case "enter":
			if m.graphifyRequested && !m.graphifyFinalized {
				m.graphifyFinalized = true
				if m.session.GraphifySymlink != "" {
					return m, graphifyCleanupCmd(m.services, m.session.GraphifySymlink)
				}
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "enter":
		if step, ok := m.currentNeedsInputStep(); ok {
			return m, openQACmd(m.services, m.session.ID, step)
		}
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	if m.createErr != "" {
		return theme.Centered(width, height, theme.ErrorText.Render("Could not start the pipeline: "+m.createErr))
	}
	if m.creating {
		return theme.Centered(width, height, m.spinner.View()+" "+theme.Dim.Render("Starting the pipeline..."))
	}
	if m.qa != nil {
		return m.qa.View(width, height)
	}

	var b strings.Builder
	b.WriteString(theme.Heading.Render("MONDAY - pipeline running") + "\n\n")

	for _, step := range m.session.Steps {
		var elapsed time.Duration
		if step.StartedAt != nil {
			elapsed = m.now.Sub(*step.StartedAt)
		}
		b.WriteString(components.ProgressRow(step, elapsed, false) + "\n")
	}
	b.WriteString("\n")

	if m.handleErr != "" {
		b.WriteString(theme.ErrorText.Render(m.handleErr) + "\n\n")
	}

	switch m.session.Status {
	case session.SessionFailed:
		b.WriteString(theme.ErrorText.Render("Pipeline failed - a pane closed unexpectedly.") + "\n\n")
		b.WriteString(components.HelpBar("q quit"))
	case session.SessionDone:
		b.WriteString(m.renderCompletion())
	default:
		if _, ok := m.currentNeedsInputStep(); ok {
			b.WriteString(theme.ErrorText.Render("A step needs input - press enter to answer.") + "\n\n")
		} else {
			b.WriteString(m.spinner.View() + " " + theme.Dim.Render("working...") + "\n\n")
		}
		b.WriteString(components.HelpBar("enter open questions (when a row needs input)", "q quit"))
	}

	return theme.Centered(width, height, b.String())
}

func (m Model) renderCompletion() string {
	if m.graphLabel == "" {
		return theme.Body.Render("Pipeline complete.") + "\n\n" + components.HelpBar("q quit")
	}
	if m.graphifyFinalized {
		return theme.Body.Render("Pipeline complete. Graph cleanup done.") + "\n\n" + components.HelpBar("q quit")
	}
	if m.graphifyRequested {
		return theme.Body.Render("Press enter once the graph update finishes in the pane.") + "\n\n" + components.HelpBar("enter confirm done", "q quit")
	}
	return theme.Body.Render("Pipeline finished. Update the graphify graph with the new context?") + "\n\n" + components.HelpBar("y yes", "n no", "q quit")
}
