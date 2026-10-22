// Package app is the one dependency-injection point (architect-plan
// section 2): Build constructs every infra implementation and injects it
// into the domain services the TUI and the notify subcommand both use.
package app

import (
	"path/filepath"

	domainagent "github.com/MayurWinjit/monday/internal/domain/agent"
	"github.com/MayurWinjit/monday/internal/domain/graphify"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
	"github.com/MayurWinjit/monday/internal/infra/claudeagents"
	"github.com/MayurWinjit/monday/internal/infra/clock"
	"github.com/MayurWinjit/monday/internal/infra/fsstore"
	"github.com/MayurWinjit/monday/internal/infra/herdr"
	"github.com/MayurWinjit/monday/internal/infra/idgen"
	"github.com/MayurWinjit/monday/internal/infra/skillwriter"
	"github.com/MayurWinjit/monday/internal/infra/toolcheck"
)

// Services is the one and only dependency-injection point: cmd/monday
// builds it once, then hands it to either the TUI (internal/tui/app.New)
// or the notify subcommand.
type Services struct {
	Store        *fsstore.Store // bootstrap + domain.Store/Backend implementations
	ToolCheck    *toolcheck.Checker
	Agents       *domainagent.Catalog
	Pipelines    *pipeline.Store
	Sessions     *session.Service
	Interactions *interaction.Service
	Graphify     *graphify.Service
	Herdr        *herdr.Client
	SkillWriter  *skillwriter.Writer
}

// Build constructs every infra implementation rooted at homeDir (normally
// os.UserHomeDir()) and wires them into the domain services.
func Build(homeDir string) (*Services, error) {
	store := fsstore.New(homeDir)
	if err := store.EnsureLayout(); err != nil {
		return nil, err
	}

	herdrClient := herdr.New()
	sysClock := clock.System{}
	toolChecker := toolcheck.New(herdrClient)

	agentDiscovery := claudeagents.New(homeDir)
	agentCatalog := domainagent.NewCatalog(agentDiscovery)

	pipelines := pipeline.NewStore(store, sysClock)
	sessions := session.NewService(store, herdrClient, sysClock, idgen.UUID{})
	interactions := interaction.NewService(store)
	graphifySvc := graphify.NewService(store, herdrClient, filepath.Join(store.Root(), "graphs"))
	skillWriter := skillwriter.New(homeDir)

	return &Services{
		Store:        store,
		ToolCheck:    toolChecker,
		Agents:       agentCatalog,
		Pipelines:    pipelines,
		Sessions:     sessions,
		Interactions: interactions,
		Graphify:     graphifySvc,
		Herdr:        herdrClient,
		SkillWriter:  skillWriter,
	}, nil
}
