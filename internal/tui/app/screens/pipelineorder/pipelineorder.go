// Package pipelineorder is the Pipeline Ordering screen (architect-plan
// section 7.3): lets the user arrange the picked agents into a run order,
// repeat an agent via an explicit "add again" action, remove a step, and
// optionally save the sequence as a named pipeline before continuing.
package pipelineorder

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	services  *app.Services
	available []agent.Agent
	sequence  []string
	cursor    int

	savePromptOpen bool
	nameInput      textinput.Model
	saveMsg        string
	saveErr        bool
}

func New(services *app.Services, available []agent.Agent, sequence []string) Model {
	ti := textinput.New()
	ti.Placeholder = "pipeline name"
	ti.CharLimit = 64
	return Model{
		services:  services,
		available: available,
		sequence:  append([]string(nil), sequence...),
		nameInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type saveResultMsg struct {
	name string
	err  error
}

func (m Model) saveCmd(name string) tea.Cmd {
	sequence := append([]string(nil), m.sequence...)
	available := m.available
	services := m.services
	return func() tea.Msg {
		_, err := services.Pipelines.Save(name, sequence, available)
		return saveResultMsg{name: name, err: err}
	}
}

func adHocPipeline(sequence []string) pipeline.Pipeline {
	return pipeline.Pipeline{Agents: append([]string(nil), sequence...)}
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case saveResultMsg:
		if msg.err != nil {
			m.saveMsg = msg.err.Error()
			m.saveErr = true
			return m, nil
		}
		m.saveMsg = fmt.Sprintf("saved as %q", msg.name)
		m.saveErr = false
		m.savePromptOpen = false
		return m, nil

	case tea.KeyMsg:
		if m.savePromptOpen {
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.nameInput.Value())
				if name == "" {
					m.saveMsg = "pipeline name must not be empty"
					m.saveErr = true
					return m, nil
				}
				return m, m.saveCmd(name)
			case "esc":
				m.savePromptOpen = false
				m.saveMsg = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.sequence)-1 {
				m.cursor++
			}
		case "shift+up":
			if m.cursor > 0 {
				m.sequence[m.cursor-1], m.sequence[m.cursor] = m.sequence[m.cursor], m.sequence[m.cursor-1]
				m.cursor--
			}
		case "shift+down":
			if m.cursor < len(m.sequence)-1 {
				m.sequence[m.cursor+1], m.sequence[m.cursor] = m.sequence[m.cursor], m.sequence[m.cursor+1]
				m.cursor++
			}
		case "a":
			if len(m.sequence) > 0 {
				m.sequence = append(append([]string(nil), m.sequence...), m.sequence[m.cursor])
			}
		case "d":
			if len(m.sequence) > 1 {
				next := append([]string(nil), m.sequence[:m.cursor]...)
				next = append(next, m.sequence[m.cursor+1:]...)
				m.sequence = next
				if m.cursor >= len(m.sequence) {
					m.cursor = len(m.sequence) - 1
				}
			}
		case "s":
			m.savePromptOpen = true
			m.saveMsg = ""
			m.nameInput.SetValue("")
			m.nameInput.Focus()
		case "enter":
			return m, tuiapp.SwitchScreen(tuiapp.ScreenRequirement, tuiapp.RequirementPayload{Pipeline: adHocPipeline(m.sequence)})
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	var b strings.Builder
	b.WriteString(theme.Heading.Render("Arrange your pipeline") + "\n\n")

	for i, name := range m.sequence {
		line := fmt.Sprintf("%d. %s", i+1, name)
		if i == m.cursor {
			line = "▸ " + lipgloss.NewStyle().Bold(true).Foreground(theme.Blue).Render(line)
		} else {
			line = "  " + theme.Body.Render(line)
		}
		b.WriteString(line + "\n")
	}

	if m.savePromptOpen {
		b.WriteString("\n" + theme.Subheading.Render("Save as: ") + m.nameInput.View() + "\n")
	}
	if m.saveMsg != "" {
		style := theme.Dim
		if m.saveErr {
			style = theme.ErrorText
		}
		b.WriteString("\n" + style.Render(m.saveMsg) + "\n")
	}

	if m.savePromptOpen {
		b.WriteString("\n" + components.HelpBar("enter confirm", "esc cancel"))
	} else {
		b.WriteString("\n" + components.HelpBar("↑/↓ move cursor", "shift+↑/↓ reorder", "a add again", "d remove", "s save as...", "enter continue", "q quit"))
	}
	return theme.Centered(width, height, b.String())
}
