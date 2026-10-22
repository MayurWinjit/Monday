package herdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/MayurWinjit/monday/internal/domain/layout"
)

// fakeExecutor is a scripted Executor for unit tests that must not depend on
// the real herdr binary. Each call consumes the next scripted response
// regardless of args, which is enough for the sequencing this package needs
// to test (e.g. SplitPane's before/after diff fallback).
type fakeExecutor struct {
	calls     [][]string
	responses []fakeResponse
	next      int
}

type fakeResponse struct {
	stdout   string
	stderr   string
	exitCode int
}

func (f *fakeExecutor) Run(args ...string) ([]byte, []byte, int, error) {
	f.calls = append(f.calls, args)
	if f.next >= len(f.responses) {
		return nil, nil, 0, fmt.Errorf("fakeExecutor: no scripted response for call %d (%v)", f.next, args)
	}
	r := f.responses[f.next]
	f.next++
	return []byte(r.stdout), []byte(r.stderr), r.exitCode, nil
}

func TestCurrentPaneID_EnvVar(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "w1:p2")
	c := NewWithExecutor(&fakeExecutor{})
	id, err := c.CurrentPaneID()
	if err != nil {
		t.Fatalf("CurrentPaneID: %v", err)
	}
	if id != "w1:p2" {
		t.Fatalf("expected w1:p2, got %s", id)
	}
}

func TestCurrentPaneID_FocusedFallback(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"panes":[{"pane_id":"w1:p1","focused":false},{"pane_id":"w1:p2","focused":true}]}}`},
	}}
	c := NewWithExecutor(ex)
	id, err := c.CurrentPaneID()
	if err != nil {
		t.Fatalf("CurrentPaneID: %v", err)
	}
	if id != "w1:p2" {
		t.Fatalf("expected w1:p2, got %s", id)
	}
}

func TestCurrentPaneID_AmbiguousOrNone(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	cases := []struct {
		name  string
		panes string
	}{
		{"none focused", `[{"pane_id":"w1:p1","focused":false}]`},
		{"two focused", `[{"pane_id":"w1:p1","focused":true},{"pane_id":"w1:p2","focused":true}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ex := &fakeExecutor{responses: []fakeResponse{
				{stdout: fmt.Sprintf(`{"id":"x","result":{"panes":%s}}`, tc.panes)},
			}}
			c := NewWithExecutor(ex)
			if _, err := c.CurrentPaneID(); err != ErrNoCurrentPane {
				t.Fatalf("expected ErrNoCurrentPane, got %v", err)
			}
		})
	}
}

func TestSplitPane_DirectParse(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"panes":[{"pane_id":"w1:p1"}]}}`},                 // before snapshot
		{stdout: `{"id":"x","result":{"pane":{"pane_id":"w1:p2"},"type":"pane_info"}}`}, // split
	}}
	c := NewWithExecutor(ex)
	newID, err := c.SplitPane("w1:p1", layout.DirectionRight)
	if err != nil {
		t.Fatalf("SplitPane: %v", err)
	}
	if newID != "w1:p2" {
		t.Fatalf("expected w1:p2, got %s", newID)
	}
}

func TestSplitPane_DiffFallbackWhenPaneIDMissing(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"panes":[{"pane_id":"w1:p1"}]}}`},                     // before snapshot
		{stdout: `{"id":"x","result":{"pane":{},"type":"pane_info"}}`},                      // split, no pane_id in result
		{stdout: `{"id":"x","result":{"panes":[{"pane_id":"w1:p1"},{"pane_id":"w1:p2"}]}}`}, // after snapshot
	}}
	c := NewWithExecutor(ex)
	newID, err := c.SplitPane("w1:p1", layout.DirectionRight)
	if err != nil {
		t.Fatalf("SplitPane: %v", err)
	}
	if newID != "w1:p2" {
		t.Fatalf("expected diff fallback to find w1:p2, got %s", newID)
	}
}

func TestStartAgent_PassesTrailingAgentFlag(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"agent":{"pane_id":"w1:p2"},"argv":["claude","--agent","architect"]}}`},
		{stdout: "", exitCode: 0}, // agent wait --until idle
	}}
	c := NewWithExecutor(ex)
	if err := c.StartAgent("w1:p2", "architect"); err != nil {
		t.Fatalf("StartAgent: %v", err)
	}
	if len(ex.calls) != 2 {
		t.Fatalf("expected 2 calls (agent start then agent wait), got %d: %v", len(ex.calls), ex.calls)
	}
	got := ex.calls[0]
	want := []string{"agent", "start", "architect", "--kind", "claude", "--pane", "w1:p2", "--", "--agent", "architect"}
	if len(got) != len(want) {
		t.Fatalf("expected args %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected args %v, got %v", want, got)
		}
	}
	if ex.calls[1][0] != "agent" || ex.calls[1][1] != "wait" {
		t.Fatalf("expected second call to be `agent wait ...`, got %v", ex.calls[1])
	}
}

func TestStartAgent_WaitFailureIsReported(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"agent":{"pane_id":"w1:p2"},"argv":["claude","--agent","architect"]}}`},
		{stderr: "timed out waiting for idle", exitCode: 1},
	}}
	c := NewWithExecutor(ex)
	if err := c.StartAgent("w1:p2", "architect"); err == nil {
		t.Fatalf("expected an error when agent wait times out")
	}
}

func TestStartAgent_FallsBackToPaneRun(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stderr: `{"id":"x","error":{"code":"unknown_command","message":"agent start unsupported"}}`, exitCode: 1},
		{stdout: "", exitCode: 0}, // pane run prints no JSON envelope on success
	}}
	c := NewWithExecutor(ex)
	if err := c.StartAgent("w1:p2", "architect"); err != nil {
		t.Fatalf("StartAgent fallback: %v", err)
	}
	if len(ex.calls) != 2 {
		t.Fatalf("expected 2 calls (agent start then pane run), got %d: %v", len(ex.calls), ex.calls)
	}
	if ex.calls[1][0] != "pane" || ex.calls[1][1] != "run" {
		t.Fatalf("expected fallback call to be `pane run ...`, got %v", ex.calls[1])
	}
}

func TestAPIErrorDecoding(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stderr: `{"id":"cli:agent:explain","error":{"code":"agent_not_found","message":"agent target w1:p9 not found"}}`, exitCode: 1},
	}}
	c := NewWithExecutor(ex)
	_, err := c.ExplainAgent("w1:p9")
	var apiErr *APIError
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "agent_not_found" {
		t.Fatalf("expected code agent_not_found, got %s", apiErr.Code)
	}
}

func TestExplainAgent_NotEnveloped(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"agent":"claude","state":"idle"}`},
	}}
	c := NewWithExecutor(ex)
	explain, err := c.ExplainAgent("w1:p2")
	if err != nil {
		t.Fatalf("ExplainAgent: %v", err)
	}
	if explain.State != "idle" {
		t.Fatalf("expected state idle, got %s", explain.State)
	}
}

func TestSendKeys_EmptyStdoutOnSuccess(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: "", exitCode: 0},
	}}
	c := NewWithExecutor(ex)
	if err := c.SendKeys("w1:p2", "enter"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
}

func TestListPanes_FiltersByTabAndPassesWorkspace(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stdout: `{"id":"x","result":{"panes":[{"pane_id":"w1:p1","tab_id":"w1:t1"},{"pane_id":"w1:p2","tab_id":"w1:t2"}]}}`},
	}}
	c := NewWithExecutor(ex)
	panes, err := c.ListPanes("w1:t2")
	if err != nil {
		t.Fatalf("ListPanes: %v", err)
	}
	if len(panes) != 1 || panes[0].PaneID != "w1:p2" {
		t.Fatalf("expected only w1:p2, got %+v", panes)
	}
	if len(ex.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(ex.calls))
	}
	args := ex.calls[0]
	found := false
	for i, a := range args {
		if a == "--workspace" && i+1 < len(args) && args[i+1] == "w1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --workspace w1 in args, got %v", args)
	}
}

func TestPromptAgent_RetriesOnTransientAgentNotFound(t *testing.T) {
	notFound := `{"id":"x","error":{"code":"agent_not_found","message":"agent target w1:p2 not found"}}`
	ex := &fakeExecutor{responses: []fakeResponse{
		{stderr: notFound, exitCode: 1},              // agent prompt: fails
		{stdout: "", exitCode: 0},                    // agent wait
		{stdout: `{"id":"x","result":{"agent":{}}}`}, // agent prompt: succeeds
	}}
	c := NewWithExecutor(ex)
	if err := c.PromptAgent("w1:p2", "hello"); err != nil {
		t.Fatalf("PromptAgent: %v", err)
	}
	if len(ex.calls) != 3 {
		t.Fatalf("expected 3 calls (prompt, wait, prompt), got %d: %v", len(ex.calls), ex.calls)
	}
}

func TestPromptAgent_GivesUpAfterMaxAttempts(t *testing.T) {
	notFound := `{"id":"x","error":{"code":"agent_not_found","message":"agent target w1:p2 not found"}}`
	responses := []fakeResponse{}
	for i := 0; i < 5; i++ {
		responses = append(responses, fakeResponse{stderr: notFound, exitCode: 1}) // agent prompt
		responses = append(responses, fakeResponse{stdout: "", exitCode: 0})       // agent wait
	}
	ex := &fakeExecutor{responses: responses}
	c := NewWithExecutor(ex)
	err := c.PromptAgent("w1:p2", "hello")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "agent_not_found" {
		t.Fatalf("expected a final agent_not_found error after exhausting retries, got %v", err)
	}
}

func TestPromptAgent_NonAgentNotFoundErrorIsNotRetried(t *testing.T) {
	ex := &fakeExecutor{responses: []fakeResponse{
		{stderr: `{"id":"x","error":{"code":"some_other_error","message":"nope"}}`, exitCode: 1},
	}}
	c := NewWithExecutor(ex)
	if err := c.PromptAgent("w1:p2", "hello"); err == nil {
		t.Fatalf("expected an error")
	}
	if len(ex.calls) != 1 {
		t.Fatalf("expected no retry for a non-agent_not_found error, got %d calls", len(ex.calls))
	}
}
