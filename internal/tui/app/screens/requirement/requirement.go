// Package requirement is the Requirement Entry screen (architect-plan
// section 7.3): a breadcrumb of the resolved pipeline above a large
// bordered textarea, with a Submit button below it.
package requirement

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	pipeline      pipeline.Pipeline
	textarea      textarea.Model
	focusOnButton bool
	err           string
}

func New(p pipeline.Pipeline) Model {
	ta := textarea.New()
	ta.Placeholder = "Describe what you want this pipeline to do..."
	ta.Focus()
	return Model{pipeline: p, textarea: ta}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) submit() (tuiapp.Screen, tea.Cmd) {
	text := strings.TrimSpace(m.textarea.Value())
	if text == "" {
		m.err = "requirement must not be empty"
		return m, nil
	}
	return m, tuiapp.SwitchScreen(tuiapp.ScreenGraphifyPrompt, tuiapp.GraphifyPromptPayload{
		Pipeline:    m.pipeline,
		Requirement: text,
	})
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			return m.submit()
		case "tab":
			m.focusOnButton = !m.focusOnButton
			if m.focusOnButton {
				m.textarea.Blur()
			} else {
				m.textarea.Focus()
			}
			return m, nil
		case "enter":
			if m.focusOnButton {
				return m.submit()
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	if m.focusOnButton {
		return m, nil
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m Model) View(width, height int) string {
	var b strings.Builder
	b.WriteString(theme.Heading.Render("Describe the task") + "\n")
	b.WriteString(theme.Subheading.Render(breadcrumb(m.pipeline)) + "\n\n")

	m.textarea.SetWidth(minInt(width-10, 80))
	m.textarea.SetHeight(minInt(height-14, 10))
	b.WriteString(theme.BorderStyle.Render(m.textarea.View()) + "\n\n")

	b.WriteString(components.Button("Submit", m.focusOnButton) + "\n")
	if m.err != "" {
		b.WriteString("\n" + theme.ErrorText.Render(m.err) + "\n")
	}
	b.WriteString("\n" + components.HelpBar("tab switch focus", "ctrl+s submit", "ctrl+c quit"))

	return theme.Centered(width, height, b.String())
}

func breadcrumb(p pipeline.Pipeline) string {
	if len(p.Agents) == 0 {
		return "(no agents)"
	}
	return strings.Join(p.Agents, " -> ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
