package agent

// Discoverer is the shape Catalog needs from an infra-level agent
// discoverer (internal/infra/claudeagents.Catalog satisfies it, as does
// domain.AgentCatalog). Defined locally rather than imported from the root
// domain package to avoid an import cycle (domain already imports this
// package for the Agent type).
type Discoverer interface {
	Discover(projectDir string) ([]Agent, error)
}

// Catalog is the thin domain-level wrapper described in architect-plan
// section 6: it delegates to a Discoverer and re-shapes the single joined
// warnings error the discoverer returns into a slice, so callers can report
// every skipped file individually without discovery ever hard-failing.
type Catalog struct {
	discoverer Discoverer
}

func NewCatalog(d Discoverer) *Catalog {
	return &Catalog{discoverer: d}
}

// Discover returns every successfully parsed agent plus one warning per
// file that was skipped. A non-empty warnings slice never means discovery
// failed - the returned agents are always valid and usable on their own.
func (c *Catalog) Discover(projectDir string) ([]Agent, []error) {
	agents, err := c.discoverer.Discover(projectDir)
	return agents, splitJoined(err)
}

// splitJoined unpacks an errors.Join-created error back into its individual
// members, so a Discoverer that aggregates per-file warnings with
// errors.Join can hand them back as a slice without a second wire format.
func splitJoined(err error) []error {
	if err == nil {
		return nil
	}
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		return u.Unwrap()
	}
	return []error{err}
}
