package session

import (
	"time"

	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

type SessionStatus string

const (
	SessionRunning SessionStatus = "running"
	SessionDone    SessionStatus = "done"
	SessionFailed  SessionStatus = "failed"
)

type StepStatus string

const (
	StepWaiting    StepStatus = "waiting"
	StepRunning    StepStatus = "running"
	StepNeedsInput StepStatus = "needs_input"
	StepDone       StepStatus = "done"
	StepFailed     StepStatus = "failed"
)

type PipelineStep struct {
	Index            int        `json:"index"`
	AgentName        string     `json:"agent_name"`
	InteractionsFile string     `json:"interactions_file"`
	PaneID           string     `json:"pane_id"`
	Status           StepStatus `json:"status"`
	Activity         string     `json:"activity,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

type Session struct {
	ID               string            `json:"id"`
	PipelineName     string            `json:"pipeline_name,omitempty"`
	Pipeline         pipeline.Pipeline `json:"pipeline"`
	Requirement      string            `json:"requirement"`
	HerdrTabID       string            `json:"herdr_tab_id"`
	MondayPaneID     string            `json:"monday_pane_id"`
	WorkAreaPaneID   string            `json:"work_area_pane_id"`
	WorkingDir       string            `json:"working_dir"`
	Steps            []PipelineStep    `json:"steps"`
	CurrentStepIndex int               `json:"current_step_index"`
	Status           SessionStatus     `json:"status"`
	GraphifyGraph    string            `json:"graphify_graph,omitempty"`
	GraphifySymlink  string            `json:"graphify_symlink,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
