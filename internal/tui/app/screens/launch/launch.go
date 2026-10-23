package launch

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/brand"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	services *app.Services

	loading   bool
	pipelines []pipeline.Pipeline
	cursor    int
	err       string

	alreadyRunningID string
	terminating      bool
	terminated       bool
	terminateErr     string
}

func New(services *app.Services, alreadyRunningID string) Model {
	return Model{services: services, loading: true, alreadyRunningID: alreadyRunningID}
}

type pipelinesLoadedMsg struct {
	pipelines []pipeline.Pipeline
	err       error
}

func loadPipelines(services *app.Services) tea.Cmd {
	return func() tea.Msg {
		pipelines, err := services.Pipelines.List()
		return pipelinesLoadedMsg{pipelines: pipelines, err: err}
	}
}

type agentsLoadedMsg struct {
	agents      []agent.Agent
	forPipeline *pipeline.Pipeline
}

func loadAgents(services *app.Services, forPipeline *pipeline.Pipeline) tea.Cmd {
	return func() tea.Msg {
		agents, _ := services.Agents.Discover("")
		return agentsLoadedMsg{agents: agents, forPipeline: forPipeline}
	}
}

func (m Model) Init() tea.Cmd {
	return loadPipelines(m.services)
}

func (m Model) showTerminateRow() bool {
	return m.alreadyRunningID != "" && !m.terminated
}

func (m Model) rowOffset() int {
	if m.showTerminateRow() {
		return 1
	}
	return 0
}

func (m Model) rowCount() int {
	return m.rowOffset() + len(m.pipelines) + 1
}

type terminateResultMsg struct {
	err error
}

func terminateSession(services *app.Services, sessionID string) tea.Cmd {
	return func() tea.Msg {
		_, err := services.Sessions.Terminate(sessionID)
		return terminateResultMsg{err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case pipelinesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.pipelines = msg.pipelines
		return m, nil

	case agentsLoadedMsg:
		if msg.forPipeline != nil {
			if err := pipeline.Validate(*msg.forPipeline, msg.agents); err != nil {
				return m, tuiapp.ShowBanner(err.Error())
			}
			return m, tuiapp.SwitchScreen(tuiapp.ScreenRequirement, tuiapp.RequirementPayload{Pipeline: *msg.forPipeline})
		}
		return m, tuiapp.SwitchScreen(tuiapp.ScreenAgentPicker, tuiapp.AgentPickerPayload{Agents: msg.agents})

	case terminateResultMsg:
		m.terminating = false
		if msg.err != nil {
			m.terminateErr = msg.err.Error()
			return m, nil
		}
		m.terminateErr = ""
		m.terminated = true
		m.cursor = 0
		return m, nil

	case tea.KeyMsg:
		if m.loading || m.err != "" {
			if msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor - 1 + m.rowCount()) % m.rowCount()
		case "down", "j":
			m.cursor = (m.cursor + 1) % m.rowCount()
		case "enter":
			if m.showTerminateRow() && m.cursor == 0 {
				if m.terminating {
					return m, nil
				}
				m.terminating = true
				m.terminateErr = ""
				return m, terminateSession(m.services, m.alreadyRunningID)
			}
			idx := m.cursor - m.rowOffset()
			if idx < len(m.pipelines) {
				p := m.pipelines[idx]
				return m, loadAgents(m.services, &p)
			}
			return m, loadAgents(m.services, nil)
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	if m.err != "" {
		return theme.Centered(width, height, theme.ErrorText.Render("Could not load saved pipelines: "+m.err))
	}
	if m.loading {
		return theme.Centered(width, height, theme.Dim.Render("Loading..."))
	}

	var b strings.Builder
	b.WriteString(theme.Heading.Render(brand.ASCII) + "\n\n")

	if m.alreadyRunningID != "" {
		if m.terminated {
			b.WriteString(theme.Body.Render("Session "+m.alreadyRunningID+" terminated.") + "\n\n")
		} else {
			b.WriteString(theme.ErrorText.Render("A Monday session is already running (session "+m.alreadyRunningID+")") + "\n")
			if m.terminateErr != "" {
				b.WriteString(theme.ErrorText.Render("  could not terminate: "+m.terminateErr) + "\n")
			}
			label := "Terminate session"
			if m.terminating {
				label = "Terminating..."
			}
			focused := m.cursor == 0
			marker := "  "
			if focused {
				marker = "▸ "
			}
			b.WriteString(marker + components.Button(label, focused) + "\n\n")
		}
	}

	offset := m.rowOffset()
	if len(m.pipelines) > 0 {
		b.WriteString(theme.Subheading.Render("Saved pipelines") + "\n")
		for i, p := range m.pipelines {
			label := fmt.Sprintf("%s  (%d agents, updated %s)", p.Name, len(p.Agents), p.UpdatedAt.Format("2006-01-02"))
			b.WriteString(renderRow(label, i+offset == m.cursor) + "\n")
		}
		b.WriteString("\n")
	}

	createLabel := "Create an orchestration"
	createFocused := m.cursor == offset+len(m.pipelines)
	marker := "  "
	if createFocused {
		marker = "▸ "
	}
	b.WriteString(marker + components.Button(createLabel, createFocused) + "\n\n")
	b.WriteString(components.HelpBar("↑/↓ move", "enter select", "q quit"))

	return theme.Centered(width, height, b.String())
}

func renderRow(label string, focused bool) string {
	marker := "  "
	style := theme.Body
	if focused {
		marker = "▸ "
		style = lipgloss.NewStyle().Bold(true).Foreground(theme.Mauve)
	}
	return marker + style.Render(label)
}
