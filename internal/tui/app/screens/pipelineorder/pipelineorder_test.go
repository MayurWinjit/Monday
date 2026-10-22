package pipelineorder

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "shift+up":
		return tea.KeyMsg{Type: tea.KeyShiftUp}
	case "shift+down":
		return tea.KeyMsg{Type: tea.KeyShiftDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func testAvailable() []agent.Agent {
	return []agent.Agent{{Name: "architect"}, {Name: "developer"}, {Name: "reviewer"}}
}

func TestView_ListsSequenceInOrder(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer", "reviewer"})
	view := m.View(80, 24)
	for _, want := range []string{"1. architect", "2. developer", "3. reviewer"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestUpdate_CursorDoesNotWrap(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer"})
	screen, _ := m.Update(key("up")) // already at 0, must stay at 0 (no wrap, unlike other screens)
	model := screen.(Model)
	if model.cursor != 0 {
		t.Fatalf("expected cursor to stay at 0, got %d", model.cursor)
	}
}

func TestUpdate_ShiftUpSwapsAndFollowsCursor(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer", "reviewer"})
	screen, _ := m.Update(key("down")) // cursor -> 1 (developer)
	model := screen.(Model)
	screen, _ = model.Update(key("shift+up"))
	model = screen.(Model)

	if model.cursor != 0 {
		t.Fatalf("expected cursor to follow the swapped item to 0, got %d", model.cursor)
	}
	want := []string{"developer", "architect", "reviewer"}
	for i, w := range want {
		if model.sequence[i] != w {
			t.Fatalf("expected sequence %v after shift+up, got %v", want, model.sequence)
		}
	}
}

func TestUpdate_AddAgainAppendsHighlightedAgent(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer"})
	screen, _ := m.Update(key("a")) // cursor 0 = architect
	model := screen.(Model)

	want := []string{"architect", "developer", "architect"}
	if len(model.sequence) != len(want) {
		t.Fatalf("expected sequence %v, got %v", want, model.sequence)
	}
	for i := range want {
		if model.sequence[i] != want[i] {
			t.Fatalf("expected sequence %v, got %v", want, model.sequence)
		}
	}
}

func TestUpdate_RemoveDeletesHighlightedEntry(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer", "reviewer"})
	screen, _ := m.Update(key("down")) // cursor -> 1 (developer)
	model := screen.(Model)
	screen, _ = model.Update(key("d"))
	model = screen.(Model)

	want := []string{"architect", "reviewer"}
	if len(model.sequence) != len(want) || model.sequence[0] != want[0] || model.sequence[1] != want[1] {
		t.Fatalf("expected sequence %v after removing developer, got %v", want, model.sequence)
	}
}

func TestUpdate_RemoveRefusesToGoBelowOneAgent(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect"})
	screen, _ := m.Update(key("d"))
	model := screen.(Model)
	if len(model.sequence) != 1 {
		t.Fatalf("expected the last agent to survive 'd', got %v", model.sequence)
	}
}

func TestUpdate_EnterWithoutSavingSwitchesToRequirementAsAdHoc(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect", "developer"})
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatalf("expected a switch-screen command")
	}
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok || switchMsg.To != tuiapp.ScreenRequirement {
		t.Fatalf("expected a SwitchScreenMsg to ScreenRequirement, got %#v", msg)
	}
	payload, ok := switchMsg.Payload.(tuiapp.RequirementPayload)
	if !ok {
		t.Fatalf("expected a RequirementPayload, got %T", switchMsg.Payload)
	}
	if payload.Pipeline.Name != "" {
		t.Fatalf("expected an ad hoc pipeline (empty Name), got %q", payload.Pipeline.Name)
	}
	if len(payload.Pipeline.Agents) != 2 {
		t.Fatalf("expected 2 agents carried through, got %v", payload.Pipeline.Agents)
	}
}

func TestUpdate_SaveFlow_OpensPromptTypesAndSubmits(t *testing.T) {
	m := New(testServicesWithStore(newFakeBackend()), testAvailable(), []string{"architect"})

	screen, _ := m.Update(key("s"))
	model := screen.(Model)
	if !model.savePromptOpen {
		t.Fatalf("expected 's' to open the save prompt")
	}

	for _, r := range "my-pipeline" {
		screen, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = screen.(Model)
	}
	if model.nameInput.Value() != "my-pipeline" {
		t.Fatalf("expected typed text in the name input, got %q", model.nameInput.Value())
	}

	screen, cmd := model.Update(key("enter"))
	model = screen.(Model)
	if cmd == nil {
		t.Fatalf("expected a save command")
	}
	msg := cmd()
	result, ok := msg.(saveResultMsg)
	if !ok {
		t.Fatalf("expected saveResultMsg, got %T", msg)
	}

	screen, _ = model.Update(result)
	model = screen.(Model)
	if model.savePromptOpen {
		t.Fatalf("expected the save prompt to close on success")
	}
	if model.saveErr {
		t.Fatalf("expected no error, got %q", model.saveMsg)
	}
	if !strings.Contains(model.saveMsg, "my-pipeline") {
		t.Fatalf("expected a confirmation mentioning the name, got %q", model.saveMsg)
	}
}

func TestUpdate_SaveFlow_DuplicateNameShowsInlineError(t *testing.T) {
	backend := newFakeBackend()
	backend.pipelines["taken"] = pipeline.Pipeline{Name: "taken"}
	model := New(testServicesWithStore(backend), testAvailable(), []string{"architect"})
	screen, _ := model.Update(key("s"))
	mm := screen.(Model)
	for _, r := range "taken" {
		screen, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		mm = screen.(Model)
	}
	screen, cmd := mm.Update(key("enter"))
	mm = screen.(Model)
	msg := cmd()

	screen, _ = mm.Update(msg)
	mm = screen.(Model)
	if !mm.saveErr {
		t.Fatalf("expected a duplicate-name error, got saveMsg=%q saveErr=%v", mm.saveMsg, mm.saveErr)
	}
	if !mm.savePromptOpen {
		t.Fatalf("expected the save prompt to stay open after a conflict")
	}
}

func TestUpdate_EscClosesSavePromptWithoutSaving(t *testing.T) {
	m := New(nil, testAvailable(), []string{"architect"})
	screen, _ := m.Update(key("s"))
	model := screen.(Model)
	screen, cmd := model.Update(key("esc"))
	model = screen.(Model)
	if model.savePromptOpen {
		t.Fatalf("expected esc to close the save prompt")
	}
	if cmd != nil {
		t.Fatalf("expected no command from esc")
	}
}

// --- fakes ---

// fakeBackend implements pipeline.Backend so pipeline.Store's real
// validation/duplicate-check/save logic runs in these tests, exactly as it
// does in internal/domain/pipeline's own tests.
type fakeBackend struct {
	pipelines map[string]pipeline.Pipeline
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{pipelines: make(map[string]pipeline.Pipeline)}
}

func (f *fakeBackend) ListPipelines() ([]pipeline.Pipeline, error) {
	out := make([]pipeline.Pipeline, 0, len(f.pipelines))
	for _, p := range f.pipelines {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeBackend) SavePipeline(p pipeline.Pipeline) error {
	f.pipelines[p.Name] = p
	return nil
}

func (f *fakeBackend) PipelineNameTaken(name string) (bool, error) {
	_, ok := f.pipelines[name]
	return ok, nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC) }

func testServicesWithStore(backend *fakeBackend) *app.Services {
	return &app.Services{Pipelines: pipeline.NewStore(backend, fixedClock{})}
}
