package session

import (
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
)

type Backend interface {
	CreateSessionDir(id string) error
	SaveSession(s Session) error
	LoadSession(id string) (Session, error)
	SaveHistory(id string, h history.History) error
	LoadHistory(id string) (history.History, error)
	SaveInteractions(sessionID, file string, f interaction.InteractionFile) error
	LoadInteractions(sessionID, file string) (interaction.InteractionFile, error)
	WriteDoc(sessionID, relPath string, content []byte) error
	DocsDir(sessionID string) string
}

type HerdrClient interface {
	ListPanes(tabID string) ([]layout.PaneInfo, error)
	SplitPane(paneID string, dir layout.Direction) (newPaneID string, err error)
	StartAgent(paneID, agentName string) error
	PromptAgent(paneID, prompt string) error
	SendKeys(paneID, keys string) error
	ReadAgent(paneID string) (string, error)
	ExplainAgent(paneID string) (layout.AgentExplain, error)
	ClosePane(paneID string) error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
