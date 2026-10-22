package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
)

// This file holds tea.Msg types (and SwitchScreenMsg payload types) shared
// across more than one screen. Screen-local messages (e.g. a single
// screen's own result/tick messages) live inside that screen's own package
// instead. A payload type has to live here too, not in either screen's own
// package: the producing and receiving screens don't import each other, so
// the only way a type assertion on SwitchScreenMsg.Payload can succeed on
// the receiving end is if both sides import the same named type from here.

// BannerMsg sets the root model's transient error banner (cleared on the
// next keypress). Screens that hit a recoverable error they want shown
// above their own view - rather than a full failure screen - return
// ShowBanner's command instead of rendering the error themselves.
type BannerMsg struct {
	Text string
}

func ShowBanner(text string) tea.Cmd {
	return func() tea.Msg { return BannerMsg{Text: text} }
}

// RequirementPayload carries the resolved pipeline into the Requirement
// screen, whether it came from a saved pipeline (Launch) or a freshly
// built ad hoc sequence (Pipeline Ordering).
type RequirementPayload struct {
	Pipeline pipeline.Pipeline
}

// AgentPickerPayload carries the discovered agent catalog into the Agent
// Picker screen.
type AgentPickerPayload struct {
	Agents []agent.Agent
}

// PipelineOrderPayload carries the discovered catalog (needed for the
// pipeline ordering screen's "add again" action) and the initial agent
// sequence (selected agents in catalog order) into the Pipeline Ordering
// screen.
type PipelineOrderPayload struct {
	Available []agent.Agent
	Sequence  []string
}

// GraphifyPromptPayload carries the resolved pipeline and submitted
// requirement text into the Graphify Prompt screen.
type GraphifyPromptPayload struct {
	Pipeline    pipeline.Pipeline
	Requirement string
}

// ProgressPayload carries everything the Progress screen needs to create a
// new session - Graph is empty when no graphify-out was injected.
type ProgressPayload struct {
	Pipeline    pipeline.Pipeline
	Requirement string
	Graph       string
}
