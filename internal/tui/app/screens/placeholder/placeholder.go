// Package placeholder is a throwaway screen used only to prove the root
// model/router/theme compile and render before any real screen exists
// (architect-plan build step 11). Step 12 replaces it as the initial
// screen with the real tool check screen.
package placeholder

import (
	tea "github.com/charmbracelet/bubbletea"

	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

type Model struct{}

func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tuiapp.Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "q" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	body := theme.Heading.Render("MONDAY") + "\n\n" +
		theme.Body.Render("TUI shell online - router and theme are wired.") + "\n\n" +
		components.HelpBar("q quit")
	return theme.Centered(width, height, body)
}
