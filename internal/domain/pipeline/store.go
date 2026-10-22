package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

// Backend is the shape Store needs from the filesystem layer. Defined
// locally (rather than importing the root domain package's Store port) to
// avoid an import cycle - domain already imports this package for the
// Pipeline type. internal/infra/fsstore.Store satisfies this structurally.
type Backend interface {
	ListPipelines() ([]Pipeline, error)
	SavePipeline(p Pipeline) error
	PipelineNameTaken(name string) (bool, error)
}

// Clock is the shape Store needs for timestamps. Mirrors domain.Clock for
// the same import-cycle reason as Backend.
type Clock interface {
	Now() time.Time
}

// Store is the domain-level pipeline use case described in architect-plan
// section 6: it owns name validation, duplicate-name rejection, agent
// resolution, and timestamping, then delegates the actual write to Backend.
type Store struct {
	backend Backend
	clock   Clock
}

func NewStore(backend Backend, clock Clock) *Store {
	return &Store{backend: backend, clock: clock}
}

func (s *Store) List() ([]Pipeline, error) {
	return s.backend.ListPipelines()
}

// Save validates name and agents against catalog, rejects a duplicate name
// outright rather than overwriting, stamps CreatedAt/UpdatedAt, and
// delegates the write to Backend.
func (s *Store) Save(name string, agents []string, catalog []agent.Agent) (Pipeline, error) {
	name = strings.TrimSpace(name)

	taken, err := s.backend.PipelineNameTaken(name)
	if err != nil {
		return Pipeline{}, fmt.Errorf("pipeline: check name taken: %w", err)
	}
	if taken {
		return Pipeline{}, fmt.Errorf("a pipeline named %q already exists", name)
	}

	now := s.clock.Now()
	p := Pipeline{
		Name:      name,
		Agents:    agents,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := Validate(p, catalog); err != nil {
		return Pipeline{}, err
	}
	if err := s.backend.SavePipeline(p); err != nil {
		return Pipeline{}, fmt.Errorf("pipeline: save %q: %w", name, err)
	}
	return p, nil
}

// Load returns the saved pipeline with the given name. There is no
// per-name backend lookup (domain.Store only exposes ListPipelines), so
// this scans the full list - saved pipeline counts are expected to be
// small (user-curated, not generated).
func (s *Store) Load(name string) (Pipeline, error) {
	pipelines, err := s.backend.ListPipelines()
	if err != nil {
		return Pipeline{}, fmt.Errorf("pipeline: load %q: %w", name, err)
	}
	for _, p := range pipelines {
		if p.Name == name {
			return p, nil
		}
	}
	return Pipeline{}, fmt.Errorf("pipeline: unknown pipeline %q", name)
}
