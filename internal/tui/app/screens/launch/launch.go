// Package launch is the Launch screen (architect-plan section 7.3 and
// REQUIREMENT.md visual requirement 1): the MONDAY ASCII banner, a list of
// saved pipelines when any exist, and a "Create an orchestration" button
// that is always the last row. The whole screen is one list - up/down
// cycles through saved pipelines then the button, enter activates
// whichever row is highlighted.
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
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

const banner = ` __  __  ___  _  _  ___    _   __   __
|  \/  |/ _ \| \| || \ \  /_\  \ \ / /
| |\/| | (_) | .  || |\ \/ _ \  \ V /
|_|  |_|\___/|_|\_||_|/_/ \_\  |_|  `

type Model struct {
	services *app.Services

	loading   bool
	pipelines []pipeline.Pipeline
	cursor    int
	err       string
}

func New(services *app.Services) Model {
	return Model{services: services, loading: true}
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

// agentsLoadedMsg carries the discovered global agent catalog back after
// either list row is activated. forPipeline is non-nil only for the
// saved-pipeline path, where it is validated against the catalog before
// continuing - if an agent file referenced by a saved pipeline was deleted
// since it was saved, this surfaces that clearly instead of failing deep
// inside session creation. Agent discovery warnings (a malformed file
// skipped) are non-fatal and not surfaced here - the agent picker screen
// shows the catalog it actually got.
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

func (m Model) rowCount() int {
	return len(m.pipelines) + 1 // +1 for "Create an orchestration"
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
			if m.cursor < len(m.pipelines) {
				p := m.pipelines[m.cursor]
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
	b.WriteString(theme.Heading.Render(banner) + "\n\n")

	if len(m.pipelines) > 0 {
		b.WriteString(theme.Subheading.Render("Saved pipelines") + "\n")
		for i, p := range m.pipelines {
			label := fmt.Sprintf("%s  (%d agents, updated %s)", p.Name, len(p.Agents), p.UpdatedAt.Format("2006-01-02"))
			b.WriteString(renderRow(label, i == m.cursor) + "\n")
		}
		b.WriteString("\n")
	}

	createLabel := "Create an orchestration"
	createFocused := m.cursor == len(m.pipelines)
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
