package layout

type PaneInfo struct {
	PaneID      string
	TabID       string
	WorkspaceID string
	CWD         string
	Focused     bool
	AgentStatus string
}

type AgentExplain struct {
	PaneID  string
	State   string
	Detail  string
	Crashed bool
}
