package graphifyprompt

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/graphify"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type phase int

const (
	phaseAskYesNo phase = iota
	phasePickGraph
)

type Model struct {
	services    *app.Services
	pipeline    pipeline.Pipeline
	requirement string

	phase          phase
	yesHighlighted bool

	loading bool
	graphs  []graphify.GraphRef
	cursor  int
	err     string
}

func New(services *app.Services, p pipeline.Pipeline, requirement string) Model {
	return Model{services: services, pipeline: p, requirement: requirement}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type graphsLoadedMsg struct {
	graphs []graphify.GraphRef
	err    error
}

func loadGraphs(services *app.Services) tea.Cmd {
	return func() tea.Msg {
		graphs, err := services.Graphify.ListGraphs()
		return graphsLoadedMsg{graphs: graphs, err: err}
	}
}

func (m Model) progressSwitch(graphLabel string) tea.Cmd {
	return tuiapp.SwitchScreen(tuiapp.ScreenProgress, tuiapp.ProgressPayload{
		Pipeline:    m.pipeline,
		Requirement: m.requirement,
		Graph:       graphLabel,
	})
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case graphsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.graphs = msg.graphs
		m.phase = phasePickGraph
		m.cursor = 0
		return m, nil

	case tea.KeyMsg:
		if m.phase == phaseAskYesNo {
			switch msg.String() {
			case "left", "right", "h", "l":
				m.yesHighlighted = !m.yesHighlighted
			case "enter":
				if m.yesHighlighted {
					m.loading = true
					return m, loadGraphs(m.services)
				}
				return m, m.progressSwitch("")
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}

		total := len(m.graphs) + 1
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor - 1 + total) % total
		case "down", "j":
			m.cursor = (m.cursor + 1) % total
		case "enter":
			if m.cursor < len(m.graphs) {
				return m, m.progressSwitch(m.graphs[m.cursor].Label)
			}
			m.phase = phaseAskYesNo
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	if m.loading {
		return theme.Centered(width, height, theme.Dim.Render("Loading saved graphs..."))
	}
	if m.err != "" {
		return theme.Centered(width, height, theme.ErrorText.Render("Could not list saved graphs: "+m.err))
	}

	var b strings.Builder
	if m.phase == phaseAskYesNo {
		b.WriteString(theme.Heading.Render("Inject a saved graphify-out into this project before starting?") + "\n\n")
		b.WriteString(components.Button("Yes", m.yesHighlighted) + "  " + components.Button("No", !m.yesHighlighted) + "\n\n")
		b.WriteString(components.HelpBar("←/→ choose", "enter select", "q quit"))
		return theme.Centered(width, height, b.String())
	}

	b.WriteString(theme.Heading.Render("Pick a saved graph") + "\n\n")
	if len(m.graphs) == 0 {
		b.WriteString(theme.Dim.Render("No saved graphify-out folders found under ~/.monday/graphs yet.") + "\n\n")
	} else {
		for i, g := range m.graphs {
			b.WriteString(renderRow(g.Label, i == m.cursor) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(renderRow("Back", m.cursor == len(m.graphs)) + "\n\n")
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
