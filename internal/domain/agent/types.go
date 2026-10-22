package agent

type AgentSource string

const (
	AgentSourceGlobal  AgentSource = "global"
	AgentSourceProject AgentSource = "project"
)

type Agent struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Source      AgentSource `json:"source"`
	FilePath    string      `json:"file_path"`
}
