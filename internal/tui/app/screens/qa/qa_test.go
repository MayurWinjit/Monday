package qa

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

type fakeInteractions struct {
	submitted map[string]string
	err       error
}

func (f *fakeInteractions) SubmitAnswers(sessionID, file string, answers map[string]string) (interaction.InteractionFile, error) {
	if f.err != nil {
		return interaction.InteractionFile{}, f.err
	}
	f.submitted = answers
	return interaction.InteractionFile{}, nil
}

type fakeSessions struct {
	resumedPane   string
	resumedPrompt string
	result        session.Session
	err           error
}

func (f *fakeSessions) ResumeStep(sessionID, paneID, resumePrompt string) (session.Session, error) {
	f.resumedPane = paneID
	f.resumedPrompt = resumePrompt
	return f.result, f.err
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func twoQuestionFile() interaction.InteractionFile {
	return interaction.InteractionFile{
		Agent: "developer",
		Questions: []interaction.Question{
			{ID: "q1", Question: "Which approach?", Options: []string{"Light", "Thorough"}},
			{ID: "q2", Question: "Need a migration?", Options: []string{"Yes", "No"}},
		},
	}
}

func TestView_ShowsQuestionProgress(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	view := m.View(80, 24)
	if !strings.Contains(view, "Question 1 of 2") {
		t.Fatalf("expected question progress, got %q", view)
	}
	if !strings.Contains(view, "Which approach?") {
		t.Fatalf("expected the question text, got %q", view)
	}
	if !strings.Contains(view, "Light") || !strings.Contains(view, "Thorough") {
		t.Fatalf("expected both options, got %q", view)
	}
	if !strings.Contains(view, "Other") {
		t.Fatalf("expected an Other row, got %q", view)
	}
}

func TestUpdate_SelectingOptionAutoAdvances(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(key("enter")) // selects optCursor 0 = "Light" for q1
	if m.answers["q1"] != "Light" {
		t.Fatalf("expected q1 answered 'Light', got %+v", m.answers)
	}
	if m.cursor != 1 {
		t.Fatalf("expected auto-advance to question 2, got cursor %d", m.cursor)
	}
}

func TestUpdate_LeftRightNavigatesQuestionsWithoutLosingAnswers(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(key("enter")) // answer q1 -> auto-advance to q2
	m, _ = m.Update(key("left"))  // back to q1
	if m.cursor != 0 {
		t.Fatalf("expected cursor back at q1, got %d", m.cursor)
	}
	if m.answers["q1"] != "Light" {
		t.Fatalf("expected q1's answer preserved after navigating away and back")
	}
}

func TestUpdate_OtherOptionOpensTextInput(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(key("down")) // optCursor -> 1 (Thorough)
	m, _ = m.Update(key("down")) // optCursor -> 2 (Other)
	m, _ = m.Update(key("enter"))
	if !m.typingOther {
		t.Fatalf("expected enter on Other to start typing")
	}

	for _, r := range "custom answer" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(key("enter")) // commit
	if m.typingOther {
		t.Fatalf("expected typingOther to close after commit")
	}
	if m.answers["q1"] != "custom answer" {
		t.Fatalf("expected q1 answered with the typed text, got %+v", m.answers)
	}
}

func TestUpdate_EmptyOtherTextDoesNotAnswer(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(key("down"))
	m, _ = m.Update(key("down")) // Other row
	m, _ = m.Update(key("enter"))
	m, _ = m.Update(key("enter")) // commit with empty text
	if _, answered := m.answers["q1"]; answered {
		t.Fatalf("expected no answer recorded for empty Other text")
	}
}

func TestUpdate_TabOnlyFocusesSubmitWhenAllAnswered(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(key("tab"))
	if m.focusOnSubmit {
		t.Fatalf("expected tab to do nothing until all questions are answered")
	}

	m, _ = m.Update(key("enter")) // answer q1
	m, _ = m.Update(key("enter")) // answer q2
	m, _ = m.Update(key("tab"))
	if !m.focusOnSubmit {
		t.Fatalf("expected tab to focus submit once all answered")
	}
}

func TestUpdate_SubmitCallsInteractionsAndResumesStep(t *testing.T) {
	interactions := &fakeInteractions{}
	sessions := &fakeSessions{result: session.Session{ID: "s1"}}
	m := New(interactions, sessions, "s1", "w1:p2", "developer", "developer", twoQuestionFile())

	m, _ = m.Update(key("enter")) // q1
	m, _ = m.Update(key("enter")) // q2
	m, _ = m.Update(key("tab"))   // focus submit

	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a submit command")
	}
	msg := cmd()
	result, ok := msg.(SubmitResultMsg)
	if !ok {
		t.Fatalf("expected SubmitResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if interactions.submitted["q1"] != "Light" || interactions.submitted["q2"] != "Yes" {
		t.Fatalf("expected both answers submitted, got %+v", interactions.submitted)
	}
	if sessions.resumedPane != "w1:p2" {
		t.Fatalf("expected ResumeStep called with the agent's pane, got %q", sessions.resumedPane)
	}
	if !strings.Contains(sessions.resumedPrompt, "developer.json") {
		t.Fatalf("expected the resume prompt to reference the interactions file, got %q", sessions.resumedPrompt)
	}
}

func TestUpdate_SubmitResultProducesSubmittedMsg(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	sess := session.Session{ID: "s1"}
	_, cmd := m.Update(SubmitResultMsg{session: sess})
	if cmd == nil {
		t.Fatalf("expected a command producing SubmittedMsg")
	}
	msg := cmd()
	submitted, ok := msg.(SubmittedMsg)
	if !ok {
		t.Fatalf("expected SubmittedMsg, got %T", msg)
	}
	if submitted.Session.ID != "s1" {
		t.Fatalf("expected the resumed session carried through")
	}
}

func TestUpdate_SubmitResultError(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	m, _ = m.Update(SubmitResultMsg{err: errors.New("boom")})
	if m.err == "" {
		t.Fatalf("expected an inline error set")
	}
}

func TestUpdate_EscProducesClosedMsg(t *testing.T) {
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", twoQuestionFile())
	_, cmd := m.Update(key("esc"))
	if cmd == nil {
		t.Fatalf("expected a command producing ClosedMsg")
	}
	if _, ok := cmd().(ClosedMsg); !ok {
		t.Fatalf("expected ClosedMsg")
	}
}

func TestNew_PreservesAlreadyAnsweredQuestions(t *testing.T) {
	answer := "Thorough"
	file := interaction.InteractionFile{
		Questions: []interaction.Question{{ID: "q1", Question: "x", Answer: &answer}},
	}
	m := New(&fakeInteractions{}, &fakeSessions{}, "s1", "w1:p2", "developer", "developer", file)
	if m.answers["q1"] != "Thorough" {
		t.Fatalf("expected pre-existing answer preserved, got %+v", m.answers)
	}
}
