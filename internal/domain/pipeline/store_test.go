package pipeline

import (
	"errors"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

type fakeBackend struct {
	pipelines    map[string]Pipeline
	saveErr      error
	nameTakenErr error
	listErr      error
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{pipelines: make(map[string]Pipeline)}
}

func (f *fakeBackend) ListPipelines() ([]Pipeline, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]Pipeline, 0, len(f.pipelines))
	for _, p := range f.pipelines {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeBackend) SavePipeline(p Pipeline) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.pipelines[p.Name] = p
	return nil
}

func (f *fakeBackend) PipelineNameTaken(name string) (bool, error) {
	if f.nameTakenErr != nil {
		return false, f.nameTakenErr
	}
	_, ok := f.pipelines[name]
	return ok, nil
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

var catalog = []agent.Agent{{Name: "architect"}, {Name: "developer"}}

func TestStoreSave_Success(t *testing.T) {
	backend := newFakeBackend()
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := NewStore(backend, fixedClock{now})

	p, err := s.Save("trio", []string{"architect", "developer"}, catalog)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if p.Name != "trio" || !p.CreatedAt.Equal(now) || !p.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected saved pipeline: %+v", p)
	}
	if _, ok := backend.pipelines["trio"]; !ok {
		t.Fatalf("expected pipeline to be persisted in backend")
	}
}

func TestStoreSave_DuplicateNameRejected(t *testing.T) {
	backend := newFakeBackend()
	backend.pipelines["trio"] = Pipeline{Name: "trio", Agents: []string{"architect"}}
	s := NewStore(backend, fixedClock{time.Now()})

	_, err := s.Save("trio", []string{"architect"}, catalog)
	if err == nil {
		t.Fatalf("expected duplicate name to be rejected")
	}
}

func TestStoreSave_UnknownAgentRejected(t *testing.T) {
	backend := newFakeBackend()
	s := NewStore(backend, fixedClock{time.Now()})

	_, err := s.Save("trio", []string{"ghostwriter"}, catalog)
	if err == nil {
		t.Fatalf("expected unknown agent to be rejected")
	}
	if len(backend.pipelines) != 0 {
		t.Fatalf("expected nothing to be persisted on validation failure")
	}
}

func TestStoreSave_PropagatesBackendErrors(t *testing.T) {
	backend := newFakeBackend()
	backend.nameTakenErr = errors.New("disk is on fire")
	s := NewStore(backend, fixedClock{time.Now()})

	if _, err := s.Save("trio", []string{"architect"}, catalog); err == nil {
		t.Fatalf("expected backend error to propagate")
	}
}

func TestStoreLoad(t *testing.T) {
	backend := newFakeBackend()
	backend.pipelines["trio"] = Pipeline{Name: "trio", Agents: []string{"architect"}}
	s := NewStore(backend, fixedClock{time.Now()})

	p, err := s.Load("trio")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "trio" {
		t.Fatalf("unexpected pipeline: %+v", p)
	}

	if _, err := s.Load("missing"); err == nil {
		t.Fatalf("expected error loading unknown pipeline")
	}
}

func TestStoreList(t *testing.T) {
	backend := newFakeBackend()
	backend.pipelines["a"] = Pipeline{Name: "a"}
	backend.pipelines["b"] = Pipeline{Name: "b"}
	s := NewStore(backend, fixedClock{time.Now()})

	pipelines, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(pipelines))
	}
}
