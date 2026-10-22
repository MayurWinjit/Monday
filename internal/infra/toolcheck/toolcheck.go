// Package toolcheck implements the startup readiness check from
// architect-plan section 7.3: herdr and claude must be on PATH, and Monday
// must be running inside an active herdr pane. It reuses
// herdr.Client.CurrentPaneID() for the pane check rather than owning that
// logic itself - see docs/developer-questions.md item 6 for why that
// dependency direction was chosen over the plan's literal wording.
package toolcheck

import "os/exec"

// HerdrClient is the narrow slice of domain.HerdrClient Checker needs.
type HerdrClient interface {
	CurrentPaneID() (string, error)
}

// Result is the outcome of one readiness check.
type Result struct {
	MissingHerdr   bool
	MissingClaude  bool
	NotInHerdrPane bool
}

// OK reports whether every requirement is satisfied.
func (r Result) OK() bool {
	return !r.MissingHerdr && !r.MissingClaude && !r.NotInHerdrPane
}

// Checker runs the readiness check. lookPath is injected (defaults to
// exec.LookPath) so tests never depend on the real PATH.
type Checker struct {
	herdr    HerdrClient
	lookPath func(string) (string, error)
}

func New(herdrClient HerdrClient) *Checker {
	return &Checker{herdr: herdrClient, lookPath: exec.LookPath}
}

// NewWithLookPath is used by tests to inject a fake PATH lookup.
func NewWithLookPath(herdrClient HerdrClient, lookPath func(string) (string, error)) *Checker {
	return &Checker{herdr: herdrClient, lookPath: lookPath}
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

	return res
}
