// Package app is the BubbleTea root: the Screen interface every screen
// implements, the ScreenID enum and SwitchScreenMsg used to move between
// them, and Model, the top-level tea.Model that owns terminal size and
// delegates everything else to the active screen.
//
// Model never imports a concrete screen package (internal/tui/app/screens/*)
// - those packages import this one (for Screen/ScreenID/SwitchScreenMsg),
// so the reverse would be an import cycle. Model is instead handed a
// ScreenFactory callback at construction time; only cmd/monday/main.go
// knows about every concrete screen type and builds that factory.
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

// Screen is implemented by every full-screen view (toolcheck, launch,
// agentpicker, pipelineorder, requirement, graphifyprompt, progress). The
// Q&A view is not a ScreenID - it is an overlay owned by the progress
// screen (architect-plan section 7.1).
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View(width, height int) string
}

type ScreenID int

const (
	ScreenToolCheck ScreenID = iota
	ScreenLaunch
	ScreenAgentPicker
	ScreenPipelineOrder
	ScreenRequirement
	ScreenGraphifyPrompt
	ScreenProgress
)

// SwitchScreenMsg is the only way screen transitions happen - a screen
// never imports another screen's package, it only ever returns this
// wrapped in a tea.Cmd.
type SwitchScreenMsg struct {
	To      ScreenID
	Payload any
}

// SwitchScreen returns a tea.Cmd producing a SwitchScreenMsg - the
// convenience constructor every screen uses instead of building the
// message literal and a closure by hand.
func SwitchScreen(to ScreenID, payload any) tea.Cmd {
	return func() tea.Msg {
		return SwitchScreenMsg{To: to, Payload: payload}
	}
}

// ScreenFactory constructs the Screen for id given payload (as produced by
// a prior SwitchScreenMsg) and the shared Services. Only cmd/monday/main.go
// builds one of these, since it is the only place allowed to import every
// concrete screen package.
type ScreenFactory func(id ScreenID, payload any, services *app.Services) Screen

type Model struct {
	services *app.Services
	factory  ScreenFactory

	width, height int
	active        Screen
	activeID      ScreenID
	banner        string // transient error banner, cleared on next keypress
}

// New builds the root model with initial already constructed as the first
// active screen (so callers can pass a screen that needs no payload, e.g.
// the tool check screen, without going through the factory once at
// startup).
func New(services *app.Services, factory ScreenFactory, initialID ScreenID, initial Screen) Model {
	return Model{
		services: services,
		factory:  factory,
		active:   initial,
		activeID: initialID,
	}
}

func (m Model) Init() tea.Cmd {
	return m.active.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.banner = ""

	case SwitchScreenMsg:
		m.activeID = msg.To
		m.active = m.factory(msg.To, msg.Payload, m.services)
		return m, m.active.Init()
	}

	screen, cmd := m.active.Update(msg)
	m.active = screen
	return m, cmd
}

func (m Model) View() string {
	if m.width > 0 && m.height > 0 && (m.width < theme.MinWidth || m.height < theme.MinHeight) {
		return theme.TooSmall(m.width, m.height)
	}

	view := m.active.View(m.width, m.height)
	if m.banner == "" {
		return view
	}
	return theme.ErrorText.Render(m.banner) + "\n" + view
}
