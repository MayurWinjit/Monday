package requirement

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
)

func typeText(m Model, s string) Model {
	for _, r := range s {
		screen, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = screen.(Model)
	}
	return m
}

func TestView_ShowsBreadcrumb(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect", "developer", "reviewer"}})
	view := m.View(80, 24)
	if !strings.Contains(view, "architect -> developer -> reviewer") {
		t.Fatalf("expected a breadcrumb of the pipeline, got %q", view)
	}
}

func TestView_EmptyPipelineBreadcrumb(t *testing.T) {
	m := New(pipeline.Pipeline{})
	view := m.View(80, 24)
	if !strings.Contains(view, "(no agents)") {
		t.Fatalf("expected a placeholder breadcrumb for an empty pipeline, got %q", view)
	}
}

func TestUpdate_TabTogglesFocus(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect"}})
	if m.focusOnButton {
		t.Fatalf("expected textarea focused initially")
	}
	screen, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = screen.(Model)
	if !m.focusOnButton {
		t.Fatalf("expected tab to move focus to the button")
	}
	screen, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = screen.(Model)
	if m.focusOnButton {
		t.Fatalf("expected a second tab to move focus back to the textarea")
	}
}

func TestUpdate_EnterInTextareaInsertsNewlineNotSubmit(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect"}})
	m = typeText(m, "line one")
	screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = screen.(Model)
	if cmd != nil {
		if _, ok := cmd().(tuiapp.SwitchScreenMsg); ok {
			t.Fatalf("expected enter while textarea is focused to insert a newline, not submit")
		}
	}
	if !strings.Contains(m.textarea.Value(), "line one") {
		t.Fatalf("expected typed text preserved, got %q", m.textarea.Value())
	}
}

func TestUpdate_EmptyRequirementRejected(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect"}})
	screen, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = screen.(Model)
	if cmd != nil {
		t.Fatalf("expected no switch command for an empty requirement")
	}
	if m.err == "" {
		t.Fatalf("expected an inline error for an empty requirement")
	}
}

func TestUpdate_CtrlSSubmitsFromAnywhere(t *testing.T) {
	m := New(pipeline.Pipeline{Name: "trio", Agents: []string{"architect", "developer"}})
	m = typeText(m, "build a widget")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatalf("expected a switch-screen command")
	}
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok || switchMsg.To != tuiapp.ScreenGraphifyPrompt {
		t.Fatalf("expected a SwitchScreenMsg to ScreenGraphifyPrompt, got %#v", msg)
	}
	payload, ok := switchMsg.Payload.(tuiapp.GraphifyPromptPayload)
	if !ok {
		t.Fatalf("expected a GraphifyPromptPayload, got %T", switchMsg.Payload)
	}
	if payload.Requirement != "build a widget" {
		t.Fatalf("expected requirement text carried through, got %q", payload.Requirement)
	}
	if payload.Pipeline.Name != "trio" {
		t.Fatalf("expected pipeline carried through, got %+v", payload.Pipeline)
	}
}

func TestUpdate_EnterOnButtonSubmits(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect"}})
	m = typeText(m, "do the thing")
	screen, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = screen.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected enter on the focused button to submit")
	}
	msg := cmd()
	if _, ok := msg.(tuiapp.SwitchScreenMsg); !ok {
		t.Fatalf("expected a SwitchScreenMsg, got %T", msg)
	}
}

func TestUpdate_WhitespaceOnlyRequirementRejected(t *testing.T) {
	m := New(pipeline.Pipeline{Agents: []string{"architect"}})
	m = typeText(m, "   ")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		t.Fatalf("expected whitespace-only requirement to be rejected")
	}
}
