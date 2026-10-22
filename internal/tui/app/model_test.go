package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
)

type fakeScreen struct {
	id       string
	viewText string
}

func (f fakeScreen) Init() tea.Cmd { return nil }

func (f fakeScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	return f, nil
}

func (f fakeScreen) View(width, height int) string {
	if f.viewText != "" {
		return f.viewText
	}
	return "screen:" + f.id
}

func TestModel_SwitchScreenMsg_ReplacesActiveScreen(t *testing.T) {
	var constructedFor ScreenID
	var constructedPayload any

	factory := func(id ScreenID, payload any, services *app.Services) Screen {
		constructedFor = id
		constructedPayload = payload
		return fakeScreen{id: "second", viewText: "second screen"}
	}

	m := New(nil, factory, ScreenToolCheck, fakeScreen{id: "first", viewText: "first screen"})

	if got := m.View(); got != "first screen" {
		t.Fatalf("expected initial screen to render, got %q", got)
	}

	updated, _ := m.Update(SwitchScreenMsg{To: ScreenLaunch, Payload: "hello"})
	m = updated.(Model)

	if constructedFor != ScreenLaunch {
		t.Fatalf("expected factory called with ScreenLaunch, got %v", constructedFor)
	}
	if constructedPayload != "hello" {
		t.Fatalf("expected payload 'hello' passed to factory, got %v", constructedPayload)
	}
	if got := m.View(); got != "second screen" {
		t.Fatalf("expected active screen replaced, got %q", got)
	}
}

func TestModel_WindowSizeMsg_TracksDimensions(t *testing.T) {
	m := New(nil, nil, ScreenToolCheck, fakeScreen{id: "only"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updated.(Model)
	if m.width != 100 || m.height != 40 {
		t.Fatalf("expected width/height tracked, got %d/%d", m.width, m.height)
	}
}

func TestModel_TooSmallFallback(t *testing.T) {
	m := New(nil, nil, ScreenToolCheck, fakeScreen{id: "only", viewText: "real view"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 10, Height: 5})
	m = updated.(Model)

	view := m.View()
	if view == "real view" {
		t.Fatalf("expected the too-small fallback for a tiny terminal, got the real screen view")
	}
}

func TestModel_CtrlCQuits(t *testing.T) {
	m := New(nil, nil, ScreenToolCheck, fakeScreen{id: "only"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected ctrl+c to produce a quit command")
	}
}
