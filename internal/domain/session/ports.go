package session

import (
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/layout"
)

// Backend, HerdrClient, Clock, and IDGenerator mirror the shapes of the
// equivalent interfaces in the root domain package (ports.go). Defined
// locally rather than imported from there to avoid an import cycle - the
// root domain package already imports this package for Store's
// session.Session parameters. internal/infra/fsstore.Store,
// internal/infra/herdr.Client, internal/infra/clock.System, and
// internal/infra/idgen all satisfy these structurally with no extra glue.
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
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
