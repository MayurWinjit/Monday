package toolcheck

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

type HerdrClient interface {
	CurrentPaneID() (string, error)
}

type Result struct {
	MissingHerdr     bool
	MissingClaude    bool
	NotInHerdrPane   bool
	AlreadyRunningID string
}

func (r Result) OK() bool {
	return !r.MissingHerdr && !r.MissingClaude && !r.NotInHerdrPane
}

type Checker struct {
	herdr    HerdrClient
	homeDir  string
	lookPath func(string) (string, error)
}

func New(herdrClient HerdrClient, homeDir string) *Checker {
	return &Checker{herdr: herdrClient, homeDir: homeDir, lookPath: exec.LookPath}
}

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
