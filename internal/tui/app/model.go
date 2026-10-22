package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

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

type SwitchScreenMsg struct {
	To      ScreenID
	Payload any
}

func SwitchScreen(to ScreenID, payload any) tea.Cmd {
	return func() tea.Msg {
		return SwitchScreenMsg{To: to, Payload: payload}
	}
}

type ScreenFactory func(id ScreenID, payload any, services *app.Services) Screen

type Model struct {
	services *app.Services
	factory  ScreenFactory

	width, height int
	active        Screen
	activeID      ScreenID
	banner        string
}

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

	case BannerMsg:
		m.banner = msg.Text
		return m, nil
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
