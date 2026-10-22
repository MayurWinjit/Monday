// Package fsstore implements the domain.Store port against the ~/.monday
// directory layout described in architect-plan.md section 5.1. Every write
// (pipeline, session, history, interactions, docs) goes through
// writeFileAtomic: write to a temp file in the same directory, then
// os.Rename over the target. A crash mid-write can therefore never leave a
// half-written JSON file behind, which is the mechanism behind the "a
// failed step must not corrupt session.json" NFR.
package fsstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/MayurWinjit/monday/internal/domain"
	"github.com/MayurWinjit/monday/internal/domain/graphify"
	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

const dirMode = 0o700

var (
	_ domain.Store        = (*Store)(nil)
	_ pipeline.Backend    = (*Store)(nil)
	_ session.Backend     = (*Store)(nil)
	_ interaction.Backend = (*Store)(nil)
	_ graphify.Backend    = (*Store)(nil)
)

// Store implements domain.Store rooted at <homeDir>/.monday. homeDir is
// injected rather than resolved internally via os.UserHomeDir so tests can
// point it at t.TempDir().
type Store struct {
	root string
}

func New(homeDir string) *Store {
	return &Store{root: filepath.Join(homeDir, ".monday")}
}

func (s *Store) Root() string         { return s.root }
func (s *Store) pipelinesDir() string { return filepath.Join(s.root, "pipelines") }
func (s *Store) sessionsDir() string  { return filepath.Join(s.root, "sessions") }
func (s *Store) graphsDir() string    { return filepath.Join(s.root, "graphs") }
func (s *Store) sessionDir(id string) string {
	return filepath.Join(s.sessionsDir(), id)
}

func (s *Store) EnsureLayout() error {
	for _, d := range []string{s.root, s.pipelinesDir(), s.sessionsDir(), s.graphsDir()} {
		if err := os.MkdirAll(d, dirMode); err != nil {
			return fmt.Errorf("fsstore: ensure layout %s: %w", d, err)
		}
	}
	return nil
}

// --- pipelines ---

func (s *Store) pipelinePath(name string) string {
	return filepath.Join(s.pipelinesDir(), name+".json")
}

func (s *Store) PipelineNameTaken(name string) (bool, error) {
	_, err := os.Stat(s.pipelinePath(name))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("fsstore: stat pipeline %s: %w", name, err)
}

func (s *Store) SavePipeline(p pipeline.Pipeline) error {
	if err := os.MkdirAll(s.pipelinesDir(), dirMode); err != nil {
		return fmt.Errorf("fsstore: ensure pipelines dir: %w", err)
	}
	return writeJSONAtomic(s.pipelinePath(p.Name), p)
}

func (s *Store) ListPipelines() ([]pipeline.Pipeline, error) {
	entries, err := os.ReadDir(s.pipelinesDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("fsstore: list pipelines: %w", err)
	}

	pipelines := make([]pipeline.Pipeline, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var p pipeline.Pipeline
		if err := readJSON(filepath.Join(s.pipelinesDir(), e.Name()), &p); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}

	sort.Slice(pipelines, func(i, j int) bool { return pipelines[i].Name < pipelines[j].Name })
	return pipelines, nil
}

// --- sessions ---

func (s *Store) CreateSessionDir(id string) error {
	dir := s.sessionDir(id)
	for _, d := range []string{dir, filepath.Join(dir, "docs"), filepath.Join(dir, "interactions")} {
		if err := os.MkdirAll(d, dirMode); err != nil {
			return fmt.Errorf("fsstore: create session dir %s: %w", d, err)
		}
	}
	return nil
}

func (s *Store) sessionPath(id string) string { return filepath.Join(s.sessionDir(id), "session.json") }
func (s *Store) historyPath(id string) string { return filepath.Join(s.sessionDir(id), "history.json") }

func (s *Store) SaveSession(sess session.Session) error {
	return writeJSONAtomic(s.sessionPath(sess.ID), sess)
}

func (s *Store) LoadSession(id string) (session.Session, error) {
	var sess session.Session
	err := readJSON(s.sessionPath(id), &sess)
	return sess, err
}

func (s *Store) SaveHistory(id string, h history.History) error {
	return writeJSONAtomic(s.historyPath(id), h)
}

func (s *Store) LoadHistory(id string) (history.History, error) {
	var h history.History
	err := readJSON(s.historyPath(id), &h)
	return h, err
}

// --- interactions ---

func (s *Store) interactionsPath(sessionID, file string) string {
	return filepath.Join(s.sessionDir(sessionID), "interactions", file+".json")
}

func (s *Store) SaveInteractions(sessionID, file string, f interaction.InteractionFile) error {
	return writeJSONAtomic(s.interactionsPath(sessionID, file), f)
}

func (s *Store) LoadInteractions(sessionID, file string) (interaction.InteractionFile, error) {
	var f interaction.InteractionFile
	err := readJSON(s.interactionsPath(sessionID, file), &f)
	return f, err
}

// --- docs ---

func (s *Store) DocsDir(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "docs")
}

func (s *Store) WriteDoc(sessionID, relPath string, content []byte) error {
	path := filepath.Join(s.DocsDir(sessionID), relPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("fsstore: ensure docs dir %s: %w", dir, err)
	}
	return writeFileAtomic(path, content)
}

// --- graphs ---

func (s *Store) ListSavedGraphs() ([]string, error) {
	entries, err := os.ReadDir(s.graphsDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("fsstore: list graphs: %w", err)
	}

	labels := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			labels = append(labels, e.Name())
		}
	}
	sort.Strings(labels)
	return labels, nil
}

// --- shared read/write helpers ---

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("fsstore: marshal %s: %w", path, err)
	}
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("fsstore: mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("fsstore: create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("fsstore: write temp %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fsstore: close temp %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("fsstore: rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("fsstore: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("fsstore: unmarshal %s: %w", path, err)
	}
	return nil
}
