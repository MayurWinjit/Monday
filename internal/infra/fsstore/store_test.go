package fsstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/history"
	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

func TestEnsureLayout(t *testing.T) {
	home := t.TempDir()
	s := New(home)

	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	for _, d := range []string{
		filepath.Join(home, ".monday"),
		filepath.Join(home, ".monday", "pipelines"),
		filepath.Join(home, ".monday", "sessions"),
		filepath.Join(home, ".monday", "graphs"),
	} {
		info, err := os.Stat(d)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", d, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", d)
		}
	}

	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout (second run): %v", err)
	}
}

func TestPipelineSaveLoadAndDuplicateName(t *testing.T) {
	s := New(t.TempDir())
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	taken, err := s.PipelineNameTaken("web-trio")
	if err != nil {
		t.Fatalf("PipelineNameTaken (before save): %v", err)
	}
	if taken {
		t.Fatalf("expected name not taken before save")
	}

	now := time.Now().Truncate(time.Second)
	p := pipeline.Pipeline{
		Name:      "web-trio",
		Agents:    []string{"architect", "developer", "reviewer"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.SavePipeline(p); err != nil {
		t.Fatalf("SavePipeline: %v", err)
	}

	taken, err = s.PipelineNameTaken("web-trio")
	if err != nil {
		t.Fatalf("PipelineNameTaken (after save): %v", err)
	}
	if !taken {
		t.Fatalf("expected name taken after save")
	}

	loaded, err := s.ListPipelines()
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(loaded))
	}
	if loaded[0].Name != p.Name || len(loaded[0].Agents) != 3 {
		t.Fatalf("loaded pipeline mismatch: %+v", loaded[0])
	}
	if !loaded[0].CreatedAt.Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, loaded[0].CreatedAt)
	}

	p.Agents = append(p.Agents, "tester")
	if err := s.SavePipeline(p); err != nil {
		t.Fatalf("SavePipeline (update): %v", err)
	}
	loaded, err = s.ListPipelines()
	if err != nil {
		t.Fatalf("ListPipelines (after update): %v", err)
	}
	if len(loaded) != 1 || len(loaded[0].Agents) != 4 {
		t.Fatalf("expected updated pipeline with 4 agents, got %+v", loaded)
	}
}

func TestListPipelinesEmpty(t *testing.T) {
	s := New(t.TempDir())
	pipelines, err := s.ListPipelines()
	if err != nil {
		t.Fatalf("ListPipelines on missing dir: %v", err)
	}
	if len(pipelines) != 0 {
		t.Fatalf("expected no pipelines, got %d", len(pipelines))
	}
}

func TestSessionHistoryInteractionsRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}

	const sessionID = "11111111-1111-1111-1111-111111111111"
	if err := s.CreateSessionDir(sessionID); err != nil {
		t.Fatalf("CreateSessionDir: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	sess := session.Session{
		ID: sessionID,
		Pipeline: pipeline.Pipeline{
			Name:   "web-trio",
			Agents: []string{"architect", "developer"},
		},
		Requirement: "build a widget",
		Steps: []session.PipelineStep{
			{Index: 0, AgentName: "architect", Status: session.StepRunning},
			{Index: 1, AgentName: "developer", Status: session.StepWaiting},
		},
		Status:    session.SessionRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	loadedSess, err := s.LoadSession(sessionID)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loadedSess.ID != sess.ID || len(loadedSess.Steps) != 2 || loadedSess.Status != session.SessionRunning {
		t.Fatalf("session round-trip mismatch: %+v", loadedSess)
	}

	h := history.History{
		Requirement: "build a widget",
		Entries: []history.HistoryEntry{
			{Timestamp: now, StepIndex: 0, AgentName: "architect", Role: history.HistoryAgent, Content: "drafted the plan"},
		},
	}
	if err := s.SaveHistory(sessionID, h); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}
	loadedHist, err := s.LoadHistory(sessionID)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(loadedHist.Entries) != 1 || loadedHist.Entries[0].Content != "drafted the plan" {
		t.Fatalf("history round-trip mismatch: %+v", loadedHist)
	}

	answer := "Option one"
	file := interaction.InteractionFile{
		Agent: "developer",
		Questions: []interaction.Question{
			{ID: "q1", Question: "Which approach?", Options: []string{"Option one", "Option two"}, Answer: &answer},
			{ID: "q2", Question: "Need a migration?"},
		},
	}
	if err := s.SaveInteractions(sessionID, "developer", file); err != nil {
		t.Fatalf("SaveInteractions: %v", err)
	}
	loadedFile, err := s.LoadInteractions(sessionID, "developer")
	if err != nil {
		t.Fatalf("LoadInteractions: %v", err)
	}
	if len(loadedFile.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(loadedFile.Questions))
	}
	if loadedFile.Questions[0].Answer == nil || *loadedFile.Questions[0].Answer != "Option one" {
		t.Fatalf("expected answer to round-trip, got %+v", loadedFile.Questions[0])
	}
	if loadedFile.Questions[1].Answer != nil {
		t.Fatalf("expected nil answer for unanswered question, got %v", *loadedFile.Questions[1].Answer)
	}

	expectedPath := filepath.Join(s.sessionDir(sessionID), "interactions", "developer.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected interactions file at %s: %v", expectedPath, err)
	}
}

func TestWriteDoc(t *testing.T) {
	s := New(t.TempDir())
	const sessionID = "22222222-2222-2222-2222-222222222222"
	if err := s.CreateSessionDir(sessionID); err != nil {
		t.Fatalf("CreateSessionDir: %v", err)
	}

	if err := s.WriteDoc(sessionID, "requirement.md", []byte("build a widget")); err != nil {
		t.Fatalf("WriteDoc: %v", err)
	}

	path := filepath.Join(s.DocsDir(sessionID), "requirement.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if string(data) != "build a widget" {
		t.Fatalf("unexpected doc content: %q", data)
	}

	if err := s.WriteDoc(sessionID, filepath.Join("reports", "plan.md"), []byte("plan")); err != nil {
		t.Fatalf("WriteDoc (nested): %v", err)
	}
	nestedPath := filepath.Join(s.DocsDir(sessionID), "reports", "plan.md")
	if _, err := os.Stat(nestedPath); err != nil {
		t.Fatalf("expected nested doc at %s: %v", nestedPath, err)
	}
}

func TestListSavedGraphs(t *testing.T) {
	home := t.TempDir()
	s := New(home)

	graphs, err := s.ListSavedGraphs()
	if err != nil {
		t.Fatalf("ListSavedGraphs on missing dir: %v", err)
	}
	if len(graphs) != 0 {
		t.Fatalf("expected no graphs, got %v", graphs)
	}

	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	for _, label := range []string{"payment-portal", "monorepo"} {
		if err := os.MkdirAll(filepath.Join(home, ".monday", "graphs", label), dirMode); err != nil {
			t.Fatalf("mkdir graph fixture: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(home, ".monday", "graphs", "README.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write stray file: %v", err)
	}

	graphs, err = s.ListSavedGraphs()
	if err != nil {
		t.Fatalf("ListSavedGraphs: %v", err)
	}
	want := []string{"monorepo", "payment-portal"}
	if len(graphs) != len(want) {
		t.Fatalf("expected %v, got %v", want, graphs)
	}
	for i := range want {
		if graphs[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, graphs)
		}
	}
}
