// Package herdr implements domain.HerdrClient by shelling out to the herdr
// CLI. See docs/developer-questions.md items 1-7 for the undocumented
// response shapes and quirks this package was built against, confirmed by
// manually exercising the real herdr binary during build step 3.
package herdr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MayurWinjit/monday/internal/domain"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

var (
	_ domain.HerdrClient  = (*Client)(nil)
	_ session.HerdrClient = (*Client)(nil)
)

// ErrNoCurrentPane is returned when Monday cannot determine the pane it is
// running in: no HERDR_PANE_ID env var, and either zero or more than one
// pane reports focused:true.
var ErrNoCurrentPane = errors.New("herdr: could not determine the current pane - launch monday from inside an active herdr pane")

// Executor abstracts process execution so the client can be tested without
// the real herdr binary.
type Executor interface {
	// Run executes `herdr <args...>` and returns its stdout, stderr, and
	// exit code. A non-zero exit code is not itself an error return value -
	// callers inspect exitCode and stderr to decode herdr's error envelope.
	Run(args ...string) (stdout, stderr []byte, exitCode int, err error)
}

type execExecutor struct{}

func (execExecutor) Run(args ...string) ([]byte, []byte, int, error) {
	cmd := exec.Command("herdr", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr == nil {
		return stdout.Bytes(), stderr.Bytes(), 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode(), nil
	}
	return stdout.Bytes(), stderr.Bytes(), -1, fmt.Errorf("herdr: exec %v: %w", args, runErr)
}

// Client implements domain.HerdrClient.
type Client struct {
	exec Executor
}

func New() *Client {
	return &Client{exec: execExecutor{}}
}

func NewWithExecutor(ex Executor) *Client {
	return &Client{exec: ex}
}

// --- wire envelope ---

type envelope[T any] struct {
	ID     string `json:"id"`
	Result T      `json:"result"`
}

// APIError wraps a decoded herdr error envelope (see developer-questions.md
// item 2: errors travel on stderr as {"id","error":{"code","message"}}).
type APIError struct {
	Args    []string
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("herdr %s: %s: %s", strings.Join(e.Args, " "), e.Code, e.Message)
}

type errorEnvelope struct {
	ID    string `json:"id"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func call[T any](ex Executor, args ...string) (T, error) {
	var zero T
	stdout, stderr, exitCode, err := ex.Run(args...)
	if err != nil {
		return zero, err
	}
	if exitCode != 0 {
		var eenv errorEnvelope
		if jsonErr := json.Unmarshal(stderr, &eenv); jsonErr == nil && eenv.Error.Code != "" {
			return zero, &APIError{Args: args, Code: eenv.Error.Code, Message: eenv.Error.Message}
		}
		return zero, fmt.Errorf("herdr %s: exit %d: %s", strings.Join(args, " "), exitCode, strings.TrimSpace(string(stderr)))
	}
	var env envelope[T]
	if jsonErr := json.Unmarshal(stdout, &env); jsonErr != nil {
		return zero, fmt.Errorf("herdr %s: decode response: %w (stdout=%s)", strings.Join(args, " "), jsonErr, bytes.TrimSpace(stdout))
	}
	return env.Result, nil
}

// --- pane wire shape ---

type wirePane struct {
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	CWD         string `json:"cwd"`
	Focused     bool   `json:"focused"`
	AgentStatus string `json:"agent_status"`
}

func (p wirePane) toDomain() layout.PaneInfo {
	return layout.PaneInfo{
		PaneID:      p.PaneID,
		TabID:       p.TabID,
		WorkspaceID: p.WorkspaceID,
		CWD:         p.CWD,
		Focused:     p.Focused,
		AgentStatus: p.AgentStatus,
	}
}

type paneListResult struct {
	Panes []wirePane `json:"panes"`
}

func (c *Client) allPanes() ([]wirePane, error) {
	res, err := call[paneListResult](c.exec, "pane", "list")
	if err != nil {
		return nil, fmt.Errorf("herdr: list panes: %w", err)
	}
	return res.Panes, nil
}

// workspaceFromTabID extracts the workspace prefix from a "wX:tY" tab ID.
func workspaceFromTabID(tabID string) string {
	if i := strings.Index(tabID, ":"); i >= 0 {
		return tabID[:i]
	}
	return ""
}

// --- current pane / tab detection (developer-questions.md item 6) ---

func (c *Client) CurrentPaneID() (string, error) {
	if id := os.Getenv("HERDR_PANE_ID"); id != "" {
		return id, nil
	}

	panes, err := c.allPanes()
	if err != nil {
		return "", fmt.Errorf("herdr: determine current pane: %w", err)
	}

	found := ""
	count := 0
	for _, p := range panes {
		if p.Focused {
			found = p.PaneID
			count++
		}
	}
	if count == 1 {
		return found, nil
	}
	return "", ErrNoCurrentPane
}

func (c *Client) CurrentTabID() (string, error) {
	if id := os.Getenv("HERDR_TAB_ID"); id != "" {
		return id, nil
	}

	paneID, err := c.CurrentPaneID()
	if err != nil {
		return "", err
	}

	panes, err := c.allPanes()
	if err != nil {
		return "", fmt.Errorf("herdr: determine current tab: %w", err)
	}
	for _, p := range panes {
		if p.PaneID == paneID {
			return p.TabID, nil
		}
	}
	return "", ErrNoCurrentPane
}

func (c *Client) ListPanes(tabID string) ([]layout.PaneInfo, error) {
	args := []string{"pane", "list"}
	if ws := workspaceFromTabID(tabID); ws != "" {
		args = append(args, "--workspace", ws)
	}

	res, err := call[paneListResult](c.exec, args...)
	if err != nil {
		return nil, fmt.Errorf("herdr: list panes for tab %s: %w", tabID, err)
	}

	out := make([]layout.PaneInfo, 0, len(res.Panes))
	for _, p := range res.Panes {
		if tabID != "" && p.TabID != tabID {
			continue
		}
		out = append(out, p.toDomain())
	}
	return out, nil
}

// --- pane split (developer-questions.md item 3) ---

type splitResult struct {
	Pane wirePane `json:"pane"`
}

func (c *Client) SplitPane(paneID string, dir layout.Direction) (string, error) {
	before, beforeErr := c.allPanes()

	res, err := call[splitResult](c.exec, "pane", "split", paneID, "--direction", string(dir))
	if err == nil && res.Pane.PaneID != "" {
		return res.Pane.PaneID, nil
	}
	if err != nil {
		return "", fmt.Errorf("herdr: split pane %s: %w", paneID, err)
	}

	// Direct-parse succeeded (exit 0, valid envelope) but result.pane.pane_id
	// was empty - fall back to a before/after diff, per architect-plan
	// section 5.2. Confirmed unnecessary against the real binary (direct
	// parse always worked), kept as a defensive secondary path.
	if beforeErr != nil {
		return "", fmt.Errorf("herdr: split pane %s: result had no pane_id and before-snapshot failed: %w", paneID, beforeErr)
	}
	after, afterErr := c.allPanes()
	if afterErr != nil {
		return "", fmt.Errorf("herdr: split pane %s: diff fallback: %w", paneID, afterErr)
	}
	beforeSet := make(map[string]bool, len(before))
	for _, p := range before {
		beforeSet[p.PaneID] = true
	}
	for _, p := range after {
		if !beforeSet[p.PaneID] {
			return p.PaneID, nil
		}
	}
	return "", fmt.Errorf("herdr: split pane %s: could not determine new pane id from either direct-parse or diff", paneID)
}

// --- agent lifecycle (developer-questions.md item 1) ---

type agentStartResult struct {
	Agent wirePane `json:"agent"`
	Argv  []string `json:"argv"`
}

// StartAgent starts `claude --agent <agentName>` in paneID via
// `herdr agent start <agentName> --kind claude --pane <paneID> -- --agent <agentName>`.
// The leading <agentName> positional is only a herdr-side display label; the
// `-- --agent <agentName>` suffix is what actually makes herdr launch that
// persona instead of a bare `claude` REPL - see developer-questions.md item 1.
func (c *Client) StartAgent(paneID, agentName string) error {
	_, err := call[agentStartResult](c.exec, "agent", "start", agentName, "--kind", "claude", "--pane", paneID, "--", "--agent", agentName)
	if err == nil {
		// agent start's response already reports interactive_ready:true, but
		// herdr's own agent registry can still lag slightly behind that -
		// confirmed during build-step-7 harness testing: an `agent prompt`
		// issued immediately after `agent start` returns (even on a
		// different, unrelated pane in between) can transiently fail with
		// agent_not_found. `agent wait --until idle` blocks until herdr's
		// own bookkeeping agrees the agent is really there, closing the race
		// before Create ever calls PromptAgent. See developer-questions.md
		// item 8.
		_, waitStderr, waitExit, waitErr := c.exec.Run("agent", "wait", paneID, "--until", "idle", "--timeout", "10000")
		if waitErr != nil {
			return fmt.Errorf("herdr: start agent %s in pane %s: agent wait: %w", agentName, paneID, waitErr)
		}
		if waitExit != 0 {
			return fmt.Errorf("herdr: start agent %s in pane %s: agent wait: exit %d: %s", agentName, paneID, waitExit, strings.TrimSpace(string(waitStderr)))
		}
		return nil
	}

	// Fallback per architect-plan section 5.2: if `agent start` is
	// unavailable, fall back to `pane run` with no trailing prompt text so
	// the agent starts idle. Not needed against the real binary (agent start
	// worked reliably) but kept as a defensive secondary path.
	//
	// `pane run`, like `pane send-keys`, prints no JSON envelope on success
	// (confirmed against the real binary during build-step-3 validation) -
	// it must not be decoded as one.
	_, stderr, exitCode, runErr := c.exec.Run("pane", "run", paneID, "claude", "--agent", agentName)
	if runErr != nil {
		return fmt.Errorf("herdr: start agent %s in pane %s: agent start failed (%v), pane run fallback failed: %w", agentName, paneID, err, runErr)
	}
	if exitCode != 0 {
		return fmt.Errorf("herdr: start agent %s in pane %s: agent start failed (%v), pane run fallback exit %d: %s", agentName, paneID, err, exitCode, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// retryOnAgentNotFound retries fn a bounded number of times, pausing on an
// `agent wait` in between, when fn fails with an agent_not_found APIError.
// Confirmed during build-step-9 harness testing: herdr's agent registry can
// transiently disagree that a pane's agent exists even after that pane's own
// StartAgent (which already waits for idle) succeeded - starting a sibling
// agent on a different pane afterward can retrigger the same race PromptAgent
// hit in step 7. See developer-questions.md item 9.
func (c *Client) retryOnAgentNotFound(paneID string, fn func() error) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		var apiErr *APIError
		if !errors.As(lastErr, &apiErr) || apiErr.Code != "agent_not_found" {
			return lastErr
		}
		c.exec.Run("agent", "wait", paneID, "--until", "idle", "--timeout", "3000")
	}
	return lastErr
}

type agentPromptResult struct {
	Agent wirePane `json:"agent"`
}

func (c *Client) PromptAgent(paneID, prompt string) error {
	return c.retryOnAgentNotFound(paneID, func() error {
		_, err := call[agentPromptResult](c.exec, "agent", "prompt", paneID, prompt)
		if err != nil {
			return fmt.Errorf("herdr: prompt pane %s: %w", paneID, err)
		}
		return nil
	})
}

func (c *Client) SendKeys(paneID, keys string) error {
	// `pane send-keys` prints no stdout body on success (confirmed against
	// the real binary) - any non-zero exit is still routed through the
	// shared error-envelope decode.
	_, _, exitCode, err := c.exec.Run("pane", "send-keys", paneID, keys)
	if err != nil {
		return fmt.Errorf("herdr: send-keys %q to pane %s: %w", keys, paneID, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("herdr: send-keys %q to pane %s: exit %d", keys, paneID, exitCode)
	}
	return nil
}

func (c *Client) ReadAgent(paneID string) (string, error) {
	var out string
	err := c.retryOnAgentNotFound(paneID, func() error {
		stdout, stderr, exitCode, err := c.exec.Run("agent", "read", paneID)
		if err != nil {
			return fmt.Errorf("herdr: read agent %s: %w", paneID, err)
		}
		if exitCode != 0 {
			var eenv errorEnvelope
			if jsonErr := json.Unmarshal(stderr, &eenv); jsonErr == nil && eenv.Error.Code != "" {
				return &APIError{Args: []string{"agent", "read", paneID}, Code: eenv.Error.Code, Message: eenv.Error.Message}
			}
			return fmt.Errorf("herdr: read agent %s: exit %d: %s", paneID, exitCode, strings.TrimSpace(string(stderr)))
		}
		// `agent read` prints raw terminal text, not a JSON envelope.
		out = string(stdout)
		return nil
	})
	return out, err
}

// --- agent explain (developer-questions.md item 4) ---

// explainResponse mirrors `herdr agent explain --json`'s top-level (non
// enveloped) output. Only the fields Monday actually uses are modeled.
type explainResponse struct {
	Agent string `json:"agent"`
	State string `json:"state"`
}

func (c *Client) ExplainAgent(paneID string) (layout.AgentExplain, error) {
	stdout, stderr, exitCode, err := c.exec.Run("agent", "explain", "--json", "--format", "json", paneID)
	if err != nil {
		return layout.AgentExplain{}, fmt.Errorf("herdr: explain agent %s: %w", paneID, err)
	}
	if exitCode != 0 {
		var eenv errorEnvelope
		if jsonErr := json.Unmarshal(stderr, &eenv); jsonErr == nil && eenv.Error.Code != "" {
			return layout.AgentExplain{}, &APIError{Args: []string{"agent", "explain", paneID}, Code: eenv.Error.Code, Message: eenv.Error.Message}
		}
		return layout.AgentExplain{}, fmt.Errorf("herdr: explain agent %s: exit %d: %s", paneID, exitCode, strings.TrimSpace(string(stderr)))
	}

	var resp explainResponse
	if jsonErr := json.Unmarshal(stdout, &resp); jsonErr != nil {
		return layout.AgentExplain{}, fmt.Errorf("herdr: explain agent %s: decode response: %w", paneID, jsonErr)
	}
	return layout.AgentExplain{
		PaneID: paneID,
		State:  resp.State,
		Detail: resp.Agent,
		// Crash/closed detection does not rely on this flag (see
		// developer-questions.md item 5) - a gone pane errors with
		// agent_not_found rather than ever reporting a "crashed" state here.
		Crashed: false,
	}, nil
}

func (c *Client) ClosePane(paneID string) error {
	_, err := call[json.RawMessage](c.exec, "pane", "close", paneID)
	if err != nil {
		return fmt.Errorf("herdr: close pane %s: %w", paneID, err)
	}
	return nil
}
