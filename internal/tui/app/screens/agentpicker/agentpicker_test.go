package agentpicker

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/agent"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func testAgents() []agent.Agent {
	return []agent.Agent{
		{Name: "architect", Description: "Plans things", Source: agent.AgentSourceGlobal},
		{Name: "developer", Description: "Builds things", Source: agent.AgentSourceProject},
		{Name: "reviewer", Description: "Reviews things", Source: agent.AgentSourceGlobal},
	}
}

func TestView_NoAgents(t *testing.T) {
	m := New(nil)
	view := m.View(80, 24)
	if !strings.Contains(view, "No agents discovered") {
		t.Fatalf("expected empty-state message, got %q", view)
	}
}

func TestView_ListsAgentsWithProjectTag(t *testing.T) {
	m := New(testAgents())
	view := m.View(80, 24)
	for _, want := range []string{"architect", "developer", "reviewer", "(project)"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestUpdate_SpaceTogglesSelection(t *testing.T) {
	m := New(testAgents())
	screen, _ := m.Update(key(" "))
	model := screen.(Model)
	if !model.selected[0] {
		t.Fatalf("expected index 0 selected after space")
	}
	screen2, _ := model.Update(key(" "))
	model2 := screen2.(Model)
	if model2.selected[0] {
		t.Fatalf("expected index 0 deselected after a second space")
	}
}

func TestUpdate_CursorWraps(t *testing.T) {
	m := New(testAgents())
	screen, _ := m.Update(key("up")) // wraps from 0 to last
	model := screen.(Model)
	if model.cursor != 2 {
		t.Fatalf("expected cursor to wrap to 2, got %d", model.cursor)
	}
}

func TestUpdate_EnterWithNoSelectionShowsBanner(t *testing.T) {
	m := New(testAgents())
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a banner command when nothing is selected")
	}
	msg := cmd()
	bannerMsg, ok := msg.(tuiapp.BannerMsg)
	if !ok {
		t.Fatalf("expected a BannerMsg, got %T", msg)
	}
	if !strings.Contains(bannerMsg.Text, "at least one") {
		t.Fatalf("expected a 'pick at least one agent' banner, got %q", bannerMsg.Text)
	}
}

func TestUpdate_EnterWithSelectionSwitchesWithOrderedSequence(t *testing.T) {
	m := New(testAgents())
	// select reviewer (idx 2) then architect (idx 0) - out of visual order
	screen, _ := m.Update(key("up")) // cursor -> 2 (reviewer)
	model := screen.(Model)
	screen, _ = model.Update(key(" ")) // select reviewer
	model = screen.(Model)
	screen, _ = model.Update(key("up")) // cursor -> 1
	model = screen.(Model)
	screen, _ = model.Update(key("up")) // cursor -> 0 (architect)
	model = screen.(Model)
	screen, _ = model.Update(key(" ")) // select architect
	model = screen.(Model)

	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a switch-screen command")
	}
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected a SwitchScreenMsg, got %T", msg)
	}
	if switchMsg.To != tuiapp.ScreenPipelineOrder {
		t.Fatalf("expected ScreenPipelineOrder, got %v", switchMsg.To)
	}
	payload, ok := switchMsg.Payload.(tuiapp.PipelineOrderPayload)
	if !ok {
		t.Fatalf("expected a PipelineOrderPayload, got %T", switchMsg.Payload)
	}
	// catalog order, not selection order: architect (0) before reviewer (2)
	if len(payload.Sequence) != 2 || payload.Sequence[0] != "architect" || payload.Sequence[1] != "reviewer" {
		t.Fatalf("expected sequence [architect, reviewer] in catalog order, got %v", payload.Sequence)
	}
	if len(payload.Available) != 3 {
		t.Fatalf("expected the full catalog passed through as Available, got %d", len(payload.Available))
	}
}

func TestUpdate_QQuits(t *testing.T) {
	m := New(testAgents())
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatalf("expected 'q' to produce a quit command")
	}
}
