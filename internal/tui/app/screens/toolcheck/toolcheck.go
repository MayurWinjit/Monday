package toolcheck

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	infratoolcheck "github.com/MayurWinjit/monday/internal/infra/toolcheck"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct {
	checker *infratoolcheck.Checker

	checked        bool
	missingHerdr   bool
	missingClaude  bool
	notInHerdrPane bool
}

func New(checker *infratoolcheck.Checker) Model {
	return Model{checker: checker}
}

type resultMsg infratoolcheck.Result

func runCheck(checker *infratoolcheck.Checker) tea.Cmd {
	return func() tea.Msg {
		return resultMsg(checker.Run())
	}
}

func (m Model) Init() tea.Cmd {
	return runCheck(m.checker)
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case resultMsg:
		m.checked = true
		m.missingHerdr = msg.MissingHerdr
		m.missingClaude = msg.MissingClaude
		m.notInHerdrPane = msg.NotInHerdrPane

		if infratoolcheck.Result(msg).OK() {
			return m, tuiapp.SwitchScreen(tuiapp.ScreenLaunch, tuiapp.LaunchPayload{AlreadyRunningID: msg.AlreadyRunningID})
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.checked = false
			return m, runCheck(m.checker)
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	if !m.checked {
		return theme.Centered(width, height, theme.Dim.Render("Checking your environment..."))
	}
	if !m.missingHerdr && !m.missingClaude && !m.notInHerdrPane {

		return theme.Centered(width, height, theme.Dim.Render("All set - continuing..."))
	}

	var b strings.Builder
	b.WriteString(theme.ErrorText.Render("Monday needs a couple of things first") + "\n\n")

	if m.missingHerdr {
		b.WriteString(theme.Body.Render("  - herdr not found on PATH") + "\n")
		b.WriteString(theme.Dim.Render("    install it: brew install herdr (see herdr docs for other platforms)") + "\n\n")
	}
	if m.missingClaude {
		b.WriteString(theme.Body.Render("  - claude not found on PATH") + "\n")
		b.WriteString(theme.Dim.Render("    install it: npm install -g @anthropic-ai/claude-code") + "\n\n")
	}
	if m.notInHerdrPane {
		b.WriteString(theme.Body.Render("  - Monday is not running inside an active herdr pane") + "\n")
		b.WriteString(theme.Dim.Render("    launch Monday from inside an active herdr pane") + "\n\n")
	}
	b.WriteString(components.HelpBar("r re-check", "q quit"))
	return theme.Centered(width, height, b.String())
}
