package requirement

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/buildinfo"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/brand"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

const (
	marginX         = 2
	minTextareaRows = 1
	maxTextareaRows = 6
	infoLabelWidth  = 10
)

type Model struct {
	pipeline      pipeline.Pipeline
	textarea      textarea.Model
	focusOnButton bool
	err           string
	cwd           string
	username      string
}

func New(p pipeline.Pipeline) Model {
	ta := textarea.New()
	ta.Placeholder = "Describe what you want this pipeline to do..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.SetHeight(minTextareaRows)
	ta.Focus()

	return Model{
		pipeline: p,
		textarea: ta,
		cwd:      currentDirectory(),
		username: currentUsername(),
	}
}

func currentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	if home, herr := os.UserHomeDir(); herr == nil && strings.HasPrefix(dir, home) {
		return "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}

func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	if v := os.Getenv("USERNAME"); v != "" {
		return v
	}
	return "unknown"
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

func (m *Model) syncTextareaHeight() {
	rows := m.textarea.LineCount()
	rows = max(rows, minTextareaRows)
	rows = min(rows, maxTextareaRows)
	m.textarea.SetHeight(rows)
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
	m.syncTextareaHeight()
	return m, cmd
}

func (m Model) View(width, height int) string {
	cw := width - marginX*2
	if cw < theme.MinWidth-marginX*2 {
		cw = max(width-marginX, 10)
	}

	footer := m.renderFooter(cw)
	footerLines := lipgloss.Height(footer)

	headerBudget := height - footerLines
	if headerBudget < 1 {
		headerBudget = 1
	}
	header := m.renderHeader(cw, headerBudget)

	headerLines := lipgloss.Height(header)
	spacerLines := height - headerLines - footerLines
	if spacerLines < 0 {
		spacerLines = 0
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for i := 0; i < spacerLines; i++ {
		b.WriteString("\n")
	}
	b.WriteString(footer)

	return lipgloss.NewStyle().PaddingLeft(marginX).Render(b.String())
}

func (m Model) renderHeader(width, heightBudget int) string {
	full := m.renderHeaderBox(width, true)
	if full != "" && lipgloss.Height(full) <= heightBudget {
		return full
	}
	return m.renderHeaderBox(width, false)
}

func (m Model) renderHeaderBox(width int, withLogo bool) string {
	boxStyle := theme.BorderStyle.Padding(0, 1)
	innerWidth := width - boxStyle.GetHorizontalFrameSize()
	if innerWidth < 10 {
		innerWidth = 10
	}

	var top string
	if withLogo {
		if brand.Width() > innerWidth {
			return ""
		}
		top = lipgloss.NewStyle().Bold(true).Foreground(theme.Mauve).Render(strings.Join(brand.Lines(), "\n"))
	} else {
		top = lipgloss.NewStyle().Bold(true).Foreground(theme.Mauve).Render("✻ Monday")
	}

	rows := append([]string{top, ""}, m.infoLines(innerWidth)...)
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return boxStyle.Width(innerWidth).Render(content)
}

func (m Model) infoLines(width int) []string {
	valueWidth := width - infoLabelWidth
	if valueWidth < 10 {
		valueWidth = 10
	}

	fields := [][2]string{
		{"Version", "v" + buildinfo.Version},
		{"Pipeline", pipelineLabel(m.pipeline)},
		{"Flow", breadcrumb(m.pipeline)},
		{"Directory", shortenPath(m.cwd, valueWidth)},
		{"User", m.username},
	}

	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		marker := theme.Dim.Render(fmt.Sprintf("%-*s", infoLabelWidth, f[0]))
		lines = append(lines, components.WrapIndent(width, marker, theme.Body, f[1]))
	}
	return lines
}

func shortenPath(path string, width int) string {
	if lipgloss.Width(path) <= width {
		return path
	}

	const ellipsis = "…/"
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		candidate := ellipsis + strings.Join(parts[i:], "/")
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	if width <= lipgloss.Width(ellipsis) {
		return ellipsis
	}
	return ellipsis + path[len(path)-(width-lipgloss.Width(ellipsis)):]
}

func pipelineLabel(p pipeline.Pipeline) string {
	if p.Name != "" {
		return p.Name
	}
	return "(unsaved)"
}

func (m Model) renderFooter(width int) string {
	boxStyle := theme.BorderStyle
	if !m.focusOnButton {
		boxStyle = theme.FocusedBorderStyle
	}
	boxStyle = boxStyle.Padding(0, 1)

	innerWidth := width - boxStyle.GetHorizontalFrameSize()
	if innerWidth < 10 {
		innerWidth = 10
	}

	ta := m.textarea
	ta.SetWidth(innerWidth)
	box := boxStyle.Width(innerWidth).Render(ta.View())

	rows := []string{box, ""}
	rows = append(rows, "  "+components.Button("Submit", m.focusOnButton))
	if m.err != "" {
		rows = append(rows, "", theme.ErrorText.Width(width).Render(m.err))
	}
	rows = append(rows, "", components.HelpBar("tab switch focus", "ctrl+s submit", "ctrl+c quit"))

	return strings.Join(rows, "\n")
}

func breadcrumb(p pipeline.Pipeline) string {
	if len(p.Agents) == 0 {
		return "(no agents)"
	}
	return strings.Join(p.Agents, " -> ")
}
