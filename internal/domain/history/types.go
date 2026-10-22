package history

import "time"

type HistoryRole string

const (
	HistoryUser   HistoryRole = "user"
	HistoryAgent  HistoryRole = "agent"
	HistorySystem HistoryRole = "system"
)

type HistoryEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	StepIndex int         `json:"step_index"`
	AgentName string      `json:"agent_name,omitempty"`
	Role      HistoryRole `json:"role"`
	Content   string      `json:"content"`
}

type History struct {
	Requirement string         `json:"requirement"`
	Entries     []HistoryEntry `json:"entries"`
}
