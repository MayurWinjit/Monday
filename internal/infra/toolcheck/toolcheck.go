// Package toolcheck implements the startup readiness check from
// architect-plan section 7.3: herdr and claude must be on PATH, and Monday
// must be running inside an active herdr pane - those three are hard stops.
// It also scans for another Monday session already marked "running"
// (challenge #8), but that condition is advisory, not a hard stop: it is
// surfaced via Result.AlreadyRunningID so the Launch screen can offer the
// user a "Terminate session" action, rather than blocking startup outright
// the way it did originally - a stale "running" status left behind by a
// crashed or force-quit Monday process otherwise had no recovery path
// short of hand-editing session.json. It reuses herdr.Client.CurrentPaneID()
// for the pane check rather than owning that logic itself - see
// docs/developer-questions.md item 6 for why that dependency direction was
// chosen over the plan's literal wording.
package toolcheck

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// HerdrClient is the narrow slice of domain.HerdrClient Checker needs.
type HerdrClient interface {
	CurrentPaneID() (string, error)
}

// Result is the outcome of one readiness check.
type Result struct {
	MissingHerdr     bool
	MissingClaude    bool
	NotInHerdrPane   bool
	AlreadyRunningID string // non-empty: another session appears to still be running (advisory, not a hard stop - see package doc)
}

// OK reports whether every hard requirement is satisfied. AlreadyRunningID
// deliberately does not factor in here - it never blocks the TUI from
// reaching the Launch screen, it is only carried along for that screen to
// display and offer to terminate.
func (r Result) OK() bool {
	return !r.MissingHerdr && !r.MissingClaude && !r.NotInHerdrPane
}

// Checker runs the readiness check. lookPath is injected (defaults to
// exec.LookPath) so tests never depend on the real PATH; homeDir is
// injected (rather than resolved via os.UserHomeDir) so tests never depend
// on the real ~/.monday.
type Checker struct {
	herdr    HerdrClient
	homeDir  string
	lookPath func(string) (string, error)
}

func New(herdrClient HerdrClient, homeDir string) *Checker {
	return &Checker{herdr: herdrClient, homeDir: homeDir, lookPath: exec.LookPath}
}

// NewWithLookPath is used by tests to inject a fake PATH lookup.
func NewWithLookPath(herdrClient HerdrClient, homeDir string, lookPath func(string) (string, error)) *Checker {
	return &Checker{herdr: herdrClient, homeDir: homeDir, lookPath: lookPath}
}

func (c *Checker) Run() Result {
	var res Result

	if _, err := c.lookPath("herdr"); err != nil {
		res.MissingHerdr = true
	}
	if _, err := c.lookPath("claude"); err != nil {
		res.MissingClaude = true
	}

	// Checking the current pane requires the herdr binary itself; skip it
	// if herdr is already known missing rather than reporting a second,
	// redundant failure mode for the same root cause.
	if !res.MissingHerdr {
		if _, err := c.herdr.CurrentPaneID(); err != nil {
			res.NotInHerdrPane = true
		}
	}

	res.AlreadyRunningID = c.findRunningSession()

	return res
}

type sessionStatusOnly struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// findRunningSession scans ~/.monday/sessions/*/session.json for one whose
// Status is "running". Unreadable or malformed session files are skipped
// silently - this check must never hard-fail startup over a single corrupt
// session directory.
func (c *Checker) findRunningSession() string {
	matches, err := filepath.Glob(filepath.Join(c.homeDir, ".monday", "sessions", "*", "session.json"))
	if err != nil {
		return ""
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s sessionStatusOnly
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		if s.Status == "running" {
			return s.ID
		}
	}
	return ""
}
