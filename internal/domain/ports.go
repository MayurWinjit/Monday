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

type HerdrClient interface {
	CurrentPaneID() (string, error)
	CurrentTabID() (string, error)
	ListPanes(tabID string) ([]layout.PaneInfo, error)
	SplitPane(paneID string, dir layout.Direction) (newPaneID string, err error)
	StartAgent(paneID, agentName string) error
	PromptAgent(paneID, prompt string) error
	SendKeys(paneID, keys string) error
	ReadAgent(paneID string) (string, error)
	ExplainAgent(paneID string) (layout.AgentExplain, error)
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
