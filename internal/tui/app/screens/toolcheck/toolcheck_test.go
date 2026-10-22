package toolcheck

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	infratoolcheck "github.com/MayurWinjit/monday/internal/infra/toolcheck"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
)

type fakeHerdr struct{ err error }

func (f fakeHerdr) CurrentPaneID() (string, error) { return "w1:p1", f.err }

func lookPathAllFound(string) (string, error) { return "/usr/bin/x", nil }

func TestInit_RunsCheck(t *testing.T) {
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected Init to return a check command")
	}
	msg := cmd()
	if _, ok := msg.(resultMsg); !ok {
		t.Fatalf("expected a resultMsg, got %T", msg)
	}
}

func TestUpdate_SuccessSwitchesToLaunchWithoutRenderingSuccessView(t *testing.T) {
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)

	_, cmd := m.Update(resultMsg(infratoolcheck.Result{}))
	if cmd == nil {
		t.Fatalf("expected a switch-screen command on success")
	}
	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected a SwitchScreenMsg, got %T", msg)
	}
	if switchMsg.To != tuiapp.ScreenLaunch {
		t.Fatalf("expected to switch to ScreenLaunch, got %v", switchMsg.To)
	}
}

func TestView_FailureState_ListsEachMissingRequirement(t *testing.T) {
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", func(name string) (string, error) {
		return "", errNotFound(name)
	})
	m := New(checker)
	screen, _ := m.Update(resultMsg(infratoolcheck.Result{MissingHerdr: true, MissingClaude: true, NotInHerdrPane: true}))

	view := screen.View(80, 24)
	for _, want := range []string{"herdr not found", "claude not found", "not running inside an active herdr pane", "r re-check", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestView_TransientOKFrame_DoesNotClaimSomethingIsMissing(t *testing.T) {
	// Regression: Update returns (m, switchScreenCmd) on success, but m
	// itself still has checked=true - bubbletea renders View() on that
	// returned model before the switch cmd's message is processed. That
	// one transient frame must not falsely show the failure heading.
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)
	screen, _ := m.Update(resultMsg(infratoolcheck.Result{}))

	view := screen.View(80, 24)
	if strings.Contains(view, "Monday needs a couple of things first") {
		t.Fatalf("expected no failure heading on the transient OK frame, got:\n%s", view)
	}
}

func TestView_BeforeFirstResult_ShowsChecking(t *testing.T) {
	m := New(New0())
	view := m.View(80, 24)
	if !strings.Contains(view, "Checking") {
		t.Fatalf("expected a checking message before the first result, got %q", view)
	}
}

func TestUpdate_RKeyTriggersRecheck(t *testing.T) {
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)
	screen, _ := m.Update(resultMsg(infratoolcheck.Result{MissingHerdr: true}))

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Fatalf("expected 'r' to trigger a re-check command")
	}
	msg := cmd()
	if _, ok := msg.(resultMsg); !ok {
		t.Fatalf("expected 'r' to re-run the check, got %T", msg)
	}
}

func TestUpdate_QKeyQuits(t *testing.T) {
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("expected 'q' to produce a quit command")
	}
}

// New0 builds a Checker whose Run() is never actually invoked in this test
// (we only check the pre-result view), so its dependencies can be trivial.
func New0() *infratoolcheck.Checker {
	return infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
}

type errNotFound string

func (e errNotFound) Error() string { return string(e) + ": not found" }

func TestUpdate_AlreadyRunningDoesNotBlock_CarriesIDToLaunch(t *testing.T) {
	// Challenge #8 revision: AlreadyRunningID alone must never hard-stop
	// here - it's forwarded to the Launch screen, which offers a
	// "Terminate session" action instead.
	checker := infratoolcheck.NewWithLookPath(fakeHerdr{}, "", lookPathAllFound)
	m := New(checker)
	screen, cmd := m.Update(resultMsg(infratoolcheck.Result{AlreadyRunningID: "abc-123"}))
	if cmd == nil {
		t.Fatalf("expected a switch-screen command even with another session running")
	}

	msg := cmd()
	switchMsg, ok := msg.(tuiapp.SwitchScreenMsg)
	if !ok {
		t.Fatalf("expected a SwitchScreenMsg, got %T", msg)
	}
	if switchMsg.To != tuiapp.ScreenLaunch {
		t.Fatalf("expected to switch to ScreenLaunch, got %v", switchMsg.To)
	}
	payload, ok := switchMsg.Payload.(tuiapp.LaunchPayload)
	if !ok || payload.AlreadyRunningID != "abc-123" {
		t.Fatalf("expected a LaunchPayload carrying the running session id, got %+v", switchMsg.Payload)
	}

	view := screen.View(80, 24)
	if strings.Contains(view, "Monday needs a couple of things first") {
		t.Fatalf("expected no failure heading when only AlreadyRunningID is set, got:\n%s", view)
	}
}
