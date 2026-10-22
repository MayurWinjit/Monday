package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

type Backend interface {
	ListPipelines() ([]Pipeline, error)
	SavePipeline(p Pipeline) error
	PipelineNameTaken(name string) (bool, error)
}

type Clock interface {
	Now() time.Time
}

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
