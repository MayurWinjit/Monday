package layout

// PaneInfo is the domain-level projection of a herdr pane, used by
// HerdrClient.ListPanes and by the crash-detection diff in
// session.Service.PollForCrashes. Lives in this leaf package (rather than
// the root domain package) so both the root domain package and
// domain/session can depend on it without an import cycle - domain/session
// needs its own HerdrClient-shaped interface for the same reason it needs
// Direction from this package.
type PaneInfo struct {
	PaneID      string
	TabID       string
	WorkspaceID string
	CWD         string
	Focused     bool
	AgentStatus string
}

// AgentExplain is the domain-level projection of `herdr agent explain --json`,
// used to distinguish a genuinely crashed/closed agent from one that is
// merely idle.
type AgentExplain struct {
	PaneID  string
	State   string
	Detail  string
	Crashed bool
}
