package graphifyprompt

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/graphify"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func testPipeline() pipeline.Pipeline {
	return pipeline.Pipeline{Name: "trio", Agents: []string{"architect", "developer"}}
}

func TestView_AskYesNo(t *testing.T) {
	m := New(nil, testPipeline(), "do the thing")
	view := m.View(80, 24)
	if !strings.Contains(view, "Inject a saved graphify-out") {
		t.Fatalf("expected the yes/no question, got %q", view)
	}
}

func TestUpdate_NoSwitchesToProgressWithEmptyGraph(t *testing.T) {
	m := New(nil, testPipeline(), "do the thing")
	// default is "No" highlighted (m.yesHighlighted starts false)
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a switch command")
	}
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok || switchMsg.To != tuiapp.ScreenProgress {
		t.Fatalf("expected a SwitchScreenMsg to ScreenProgress, got %#v", msg)
	}
	payload, ok := switchMsg.Payload.(tuiapp.ProgressPayload)
	if !ok {
		t.Fatalf("expected a ProgressPayload, got %T", switchMsg.Payload)
	}
	if payload.Graph != "" {
		t.Fatalf("expected empty graph for the 'No' path, got %q", payload.Graph)
	}
	if payload.Requirement != "do the thing" {
		t.Fatalf("expected requirement carried through, got %q", payload.Requirement)
	}
}

func TestUpdate_LeftRightTogglesChoice(t *testing.T) {
	m := New(nil, testPipeline(), "x")
	if m.yesHighlighted {
		t.Fatalf("expected 'No' highlighted by default")
	}
	screen, _ := m.Update(key("left"))
	model := screen.(Model)
	if !model.yesHighlighted {
		t.Fatalf("expected left/right to toggle to 'Yes'")
	}
}

func TestUpdate_YesTriggersGraphLoad(t *testing.T) {
	m := New(testServices(), testPipeline(), "x")
	screen, _ := m.Update(key("left")) // toggle to Yes
	model := screen.(Model)
	_, cmd := model.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a load-graphs command")
	}
	msg := cmd()
	if _, ok := msg.(graphsLoadedMsg); !ok {
		t.Fatalf("expected graphsLoadedMsg, got %T", msg)
	}
}

func TestUpdate_GraphsLoaded_EmptyShowsBackOnly(t *testing.T) {
	m := New(testServices(), testPipeline(), "x")
	screen, _ := m.Update(graphsLoadedMsg{graphs: nil})
	model := screen.(Model)
	view := model.View(80, 24)
	if !strings.Contains(view, "No saved graphify-out folders found") {
		t.Fatalf("expected the empty-state message, got %q", view)
	}
	if !strings.Contains(view, "Back") {
		t.Fatalf("expected a Back option, got %q", view)
	}
}

func TestUpdate_PickGraph_EnterSwitchesWithLabel(t *testing.T) {
	m := New(testServices(), testPipeline(), "x")
	screen, _ := m.Update(graphsLoadedMsg{graphs: []graphify.GraphRef{{Label: "payment-portal"}, {Label: "monorepo"}}})
	model := screen.(Model)

	_, cmd := model.Update(key("enter")) // cursor 0 = payment-portal
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok || switchMsg.To != tuiapp.ScreenProgress {
		t.Fatalf("expected a SwitchScreenMsg to ScreenProgress, got %#v", msg)
	}
	payload := switchMsg.Payload.(tuiapp.ProgressPayload)
	if payload.Graph != "payment-portal" {
		t.Fatalf("expected graph 'payment-portal', got %q", payload.Graph)
	}
}

func TestUpdate_PickGraph_BackReturnsToAskYesNo(t *testing.T) {
	m := New(testServices(), testPipeline(), "x")
	screen, _ := m.Update(graphsLoadedMsg{graphs: []graphify.GraphRef{{Label: "payment-portal"}}})
	model := screen.(Model)

	// cursor: 0=payment-portal, 1=Back
	screen, _ = model.Update(key("down"))
	model = screen.(Model)
	screen, cmd := model.Update(key("enter"))
	model = screen.(Model)
	if cmd != nil {
		t.Fatalf("expected Back to produce no command, just a phase change")
	}
	if model.phase != phaseAskYesNo {
		t.Fatalf("expected Back to return to phaseAskYesNo")
	}
}

func TestUpdate_PickGraph_CursorWraps(t *testing.T) {
	m := New(testServices(), testPipeline(), "x")
	screen, _ := m.Update(graphsLoadedMsg{graphs: []graphify.GraphRef{{Label: "a"}, {Label: "b"}}})
	model := screen.(Model)
	screen, _ = model.Update(key("up")) // wraps from 0 to last (2 graphs + Back = 3 rows)
	model = screen.(Model)
	if model.cursor != 2 {
		t.Fatalf("expected cursor to wrap to 2, got %d", model.cursor)
	}
}

func testServices() *app.Services {
	return &app.Services{Graphify: graphify.NewService(fakeBackend{}, nil, "/x")}
}

type fakeBackend struct{}

func (fakeBackend) ListSavedGraphs() ([]string, error) { return nil, nil }
