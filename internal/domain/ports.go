package domain

import (
	"time"

	"github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

// PaneInfo is the domain-level projection of a herdr pane, used by
// HerdrClient.ListPanes and by the crash-detection diff in
// session.Service.PollForCrashes.
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

type HerdrClient interface {
	CurrentPaneID() (string, error)
	CurrentTabID() (string, error)
	ListPanes(tabID string) ([]PaneInfo, error)
	SplitPane(paneID string, dir layout.Direction) (newPaneID string, err error)
	StartAgent(paneID, agentName string) error
	PromptAgent(paneID, prompt string) error
	SendKeys(paneID, keys string) error
	ReadAgent(paneID string) (string, error)
	ExplainAgent(paneID string) (AgentExplain, error)
	ClosePane(paneID string) error
}

type Store interface {
	EnsureLayout() error
	ListPipelines() ([]pipeline.Pipeline, error)
	SavePipeline(p pipeline.Pipeline) error
	PipelineNameTaken(name string) (bool, error)
	CreateSessionDir(id string) error
	SaveSession(s session.Session) error
	LoadSession(id string) (session.Session, error)
	SaveHistory(id string, h history.History) error
	LoadHistory(id string) (history.History, error)
	SaveInteractions(sessionID, file string, f interaction.InteractionFile) error
	LoadInteractions(sessionID, file string) (interaction.InteractionFile, error)
	DocsDir(sessionID string) string
	WriteDoc(sessionID, relPath string, content []byte) error
	ListSavedGraphs() ([]string, error)
}

type NotifyServer interface {
	Listen(sessionID string) (events <-chan notifyproto.NotifyMessage, closeFn func() error, err error)
}

type NotifyClient interface {
	Send(socketPath string, msg notifyproto.NotifyMessage) (notifyproto.NotifyAck, error)
}

type AgentCatalog interface {
	Discover(projectDir string) ([]agent.Agent, error)
}

type SkillInstaller interface {
	Install() error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
