package agentpicker

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/domain/agent"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	agents   []agent.Agent
	cursor   int
	selected map[int]bool
}

func New(agents []agent.Agent) Model {
	return Model{agents: agents, selected: map[int]bool{}}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || len(m.agents) == 0 {
		if ok && keyMsg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.cursor = (m.cursor - 1 + len(m.agents)) % len(m.agents)
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(m.agents)
	case " ":
		sel := make(map[int]bool, len(m.selected)+1)
		for k, v := range m.selected {
			sel[k] = v
		}
		sel[m.cursor] = !sel[m.cursor]
		m.selected = sel
	case "enter":
		idx := selectedIndices(m.selected)
		if len(idx) == 0 {
			return m, tuiapp.ShowBanner("pick at least one agent")
		}
		sequence := make([]string, len(idx))
		for i, ix := range idx {
			sequence[i] = m.agents[ix].Name
		}
		return m, tuiapp.SwitchScreen(tuiapp.ScreenPipelineOrder, tuiapp.PipelineOrderPayload{
			Available: m.agents,
			Sequence:  sequence,
		})
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func selectedIndices(selected map[int]bool) []int {
	idx := make([]int, 0, len(selected))
	for i, v := range selected {
		if v {
			idx = append(idx, i)
		}
	}
	sort.Ints(idx)
	return idx
}

func (m Model) View(width, height int) string {
	if len(m.agents) == 0 {
		body := theme.ErrorText.Render("No agents discovered.") + "\n" +
			theme.Dim.Render("Add a Claude Code agent under ~/.claude/agents or <project>/.claude/agents.") + "\n\n" +
			components.HelpBar("q quit")
		return theme.Centered(width, height, body)
	}

	descWidth := width - 40
	if descWidth < 10 {
		descWidth = 30
	}

	var b strings.Builder
	b.WriteString(theme.Heading.Render("Pick agents for your pipeline") + "\n\n")
	for i, a := range m.agents {
		checkbox := "[ ]"
		if m.selected[i] {
			checkbox = "[x]"
		}

		tag := ""
		if a.Source == agent.AgentSourceProject {
			tag = " " + lipgloss.NewStyle().Foreground(theme.Peach).Render("(project)")
		}

		desc := a.Description
		if len(desc) > descWidth {
			desc = desc[:descWidth-3] + "..."
		}

		line := fmt.Sprintf("%s %s%s - %s", checkbox, a.Name, tag, theme.Dim.Render(desc))
		marker := "  "
		if i == m.cursor {
			marker = "▸ "
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(marker + line + "\n")
	}
	b.WriteString("\n" + components.HelpBar("↑/↓ move", "space toggle", "enter continue", "q quit"))
	return theme.Centered(width, height, b.String())
}
