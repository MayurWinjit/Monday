// Package qa is the Q&A overlay (architect-plan section 7.3), embedded in
// the Progress screen rather than being a routable ScreenID (section 7.1):
// it is only ever reachable from, and always returns to, the progress
// view. Model is a small self-contained component with its own Update/View
// - Progress forwards every key to it while it is open and reacts to the
// SubmittedMsg/ClosedMsg it produces.
package qa

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/session"
	"github.com/MayurWinjit/monday/internal/tui/components"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

// Interactions is the narrow slice of interaction.Service Model needs.
type Interactions interface {
	SubmitAnswers(sessionID, file string, answers map[string]string) (interaction.InteractionFile, error)
}

// Sessions is the narrow slice of session.Service Model needs to resume the
// agent once every question has been answered.
type Sessions interface {
	ResumeStep(sessionID, paneID, resumePrompt string) (session.Session, error)
}

// SubmittedMsg is produced once submitted answers have been persisted and
// the agent resumed. Progress's Update clears its overlay and replaces its
// session with Session on this message.
type SubmittedMsg struct {
	Session session.Session
	Err     error
}

// ClosedMsg indicates the user dismissed the overlay (esc) without
// submitting - answers already picked are kept in Model, not lost, in case
// Progress reopens the same Model via enter before the agent times out
// (there is no server-side timeout to race against, since the agent is
// simply idle, not polling).
type ClosedMsg struct{}

type Model struct {
	interactions Interactions
	sessions     Sessions

	sessionID string
	paneID    string
	agentName string
	file      string // interactions file stem, no .json

	questions []interaction.Question
	cursor    int // which question
	optCursor int // which option row; last index is "Other"
	answers   map[string]string

	other       textinput.Model
	typingOther bool

	focusOnSubmit bool
	submitting    bool
	err           string
}

func New(interactions Interactions, sessions Sessions, sessionID, paneID, agentName, file string, qfile interaction.InteractionFile) Model {
	ti := textinput.New()
	ti.Placeholder = "type your own answer"
	ti.CharLimit = 200

	answers := make(map[string]string, len(qfile.Questions))
	for _, q := range qfile.Questions {
		if q.Answer != nil {
			answers[q.ID] = *q.Answer
		}
	}

	return Model{
		interactions: interactions,
		sessions:     sessions,
		sessionID:    sessionID,
		paneID:       paneID,
		agentName:    agentName,
		file:         file,
		questions:    qfile.Questions,
		answers:      answers,
		other:        ti,
	}
}

func (m Model) currentQuestion() interaction.Question {
	return m.questions[m.cursor]
}

func (m Model) optRowCount() int {
	return len(m.currentQuestion().Options) + 1 // +1 for "Other"
}

func (m Model) allAnswered() bool {
	for _, q := range m.questions {
		if _, ok := m.answers[q.ID]; !ok {
			return false
		}
	}
	return true
}

func (m Model) answeredCount() int {
	n := 0
	for _, q := range m.questions {
		if _, ok := m.answers[q.ID]; ok {
			n++
		}
	}
	return n
}

// nextUnansweredCursor finds the next question (starting after the
// current one, wrapping) that has no answer yet. If every question is
// already answered, the cursor is left where it is.
func (m Model) nextUnansweredCursor() int {
	for i := 1; i <= len(m.questions); i++ {
		idx := (m.cursor + i) % len(m.questions)
		if _, ok := m.answers[m.questions[idx].ID]; !ok {
			return idx
		}
	}
	return m.cursor
}

type SubmitResultMsg struct {
	session session.Session
	err     error
}

func (m Model) submitCmd() tea.Cmd {
	interactions := m.interactions
	sessions := m.sessions
	sessionID := m.sessionID
	paneID := m.paneID
	agentName := m.agentName
	file := m.file
	answers := make(map[string]string, len(m.answers))
	for k, v := range m.answers {
		answers[k] = v
	}

	return func() tea.Msg {
		if _, err := interactions.SubmitAnswers(sessionID, file, answers); err != nil {
			return SubmitResultMsg{err: fmt.Errorf("submit answers: %w", err)}
		}
		prompt := interaction.ResumePrompt(agentName, file)
		sess, err := sessions.ResumeStep(sessionID, paneID, prompt)
		if err != nil {
			return SubmitResultMsg{err: fmt.Errorf("resume step: %w", err)}
		}
		return SubmitResultMsg{session: sess}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SubmitResultMsg:
		m.submitting = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		sess := msg.session
		return m, func() tea.Msg { return SubmittedMsg{Session: sess} }

	case tea.KeyMsg:
		if m.typingOther {
			switch msg.String() {
			case "enter":
				if val := strings.TrimSpace(m.other.Value()); val != "" {
					m.answers[m.currentQuestion().ID] = val
				}
				m.typingOther = false
				m.other.Blur()
				m.cursor = m.nextUnansweredCursor()
				m.optCursor = 0
				return m, nil
			case "esc":
				m.typingOther = false
				m.other.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.other, cmd = m.other.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return ClosedMsg{} }
		case "tab":
			if m.allAnswered() {
				m.focusOnSubmit = !m.focusOnSubmit
			}
		case "up":
			if !m.focusOnSubmit {
				m.optCursor = (m.optCursor - 1 + m.optRowCount()) % m.optRowCount()
			}
		case "down":
			if !m.focusOnSubmit {
				m.optCursor = (m.optCursor + 1) % m.optRowCount()
			}
		case "left":
			if !m.focusOnSubmit && m.cursor > 0 {
				m.cursor--
				m.optCursor = 0
			}
		case "right":
			if !m.focusOnSubmit && m.cursor < len(m.questions)-1 {
				m.cursor++
				m.optCursor = 0
			}
		case "enter":
			if m.focusOnSubmit {
				if m.allAnswered() && !m.submitting {
					m.submitting = true
					return m, m.submitCmd()
				}
				return m, nil
			}
			q := m.currentQuestion()
			if m.optCursor == len(q.Options) {
				m.typingOther = true
				m.other.SetValue("")
				m.other.Focus()
				return m, nil
			}
			m.answers[q.ID] = q.Options[m.optCursor]
			m.cursor = m.nextUnansweredCursor()
			m.optCursor = 0
		}
	}
	return m, nil
}

func (m Model) View(width, height int) string {
	q := m.currentQuestion()

	var b strings.Builder
	b.WriteString(theme.Dim.Render(fmt.Sprintf("Question %d of %d", m.cursor+1, len(m.questions))) + "\n\n")
	b.WriteString(theme.Heading.Render(q.Question) + "\n")
	if q.Description != "" {
		b.WriteString(theme.Dim.Render(q.Description) + "\n")
	}
	b.WriteString("\n")

	rows := append(append([]string(nil), q.Options...), "Other (type your own)")
	for i, label := range rows {
		isOther := i == len(rows)-1
		focused := i == m.optCursor && !m.focusOnSubmit

		marker := "  "
		style := theme.Body
		if focused {
			marker = "▸ "
			style = lipgloss.NewStyle().Bold(true).Foreground(theme.Mauve)
		}

		if isOther && m.typingOther {
			b.WriteString(marker + m.other.View() + "\n")
			continue
		}
		if isOther {
			if answer, ok := m.answers[q.ID]; ok && !isOption(answer, q.Options) {
				label = fmt.Sprintf("Other: %s", answer)
			}
		}
		b.WriteString(marker + style.Render(label) + "\n")
	}

	b.WriteString("\n" + theme.Dim.Render(fmt.Sprintf("%d of %d answered", m.answeredCount(), len(m.questions))) + "\n\n")

	b.WriteString(components.Button("Submit answers", m.focusOnSubmit) + "\n")
	if m.err != "" {
		b.WriteString("\n" + theme.ErrorText.Render(m.err) + "\n")
	}
	b.WriteString("\n" + components.HelpBar("↑/↓ option", "←/→ question", "enter select", "tab submit (once all answered)", "esc close"))

	return theme.Centered(width, height, b.String())
}

func isOption(answer string, options []string) bool {
	for _, o := range options {
		if o == answer {
			return true
		}
	}
	return false
}
