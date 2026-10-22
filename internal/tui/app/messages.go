package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

type BannerMsg struct {
	Text string
}

func ShowBanner(text string) tea.Cmd {
	return func() tea.Msg { return BannerMsg{Text: text} }
}

type LaunchPayload struct {
	AlreadyRunningID string
}

type RequirementPayload struct {
	Pipeline pipeline.Pipeline
}

type AgentPickerPayload struct {
	Agents []agent.Agent
}

type PipelineOrderPayload struct {
	Available []agent.Agent
	Sequence  []string
}

type GraphifyPromptPayload struct {
	Pipeline    pipeline.Pipeline
	Requirement string
}

type ProgressPayload struct {
	Pipeline    pipeline.Pipeline
	Requirement string
	Graph       string
}
