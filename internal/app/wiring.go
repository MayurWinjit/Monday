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

type Services struct {
	Store        *fsstore.Store
	ToolCheck    *toolcheck.Checker
	Agents       *domainagent.Catalog
	Pipelines    *pipeline.Store
	Sessions     *session.Service
	Interactions *interaction.Service
	Graphify     *graphify.Service
	Herdr        *herdr.Client
	SkillWriter  *skillwriter.Writer
}

func Build(homeDir string) (*Services, error) {
	store := fsstore.New(homeDir)
	if err := store.EnsureLayout(); err != nil {
		return nil, err
	}

	herdrClient := herdr.New()
	sysClock := clock.System{}
	toolChecker := toolcheck.New(herdrClient, homeDir)

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
