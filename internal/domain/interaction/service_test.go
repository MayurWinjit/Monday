package interaction

import (
	"fmt"
	"testing"
)

type fakeBackend struct {
	files map[string]InteractionFile
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{files: make(map[string]InteractionFile)}
}

func (f *fakeBackend) key(sessionID, file string) string { return sessionID + "/" + file }

func (f *fakeBackend) LoadInteractions(sessionID, file string) (InteractionFile, error) {
	ifile, ok := f.files[f.key(sessionID, file)]
	if !ok {
		return InteractionFile{}, fmt.Errorf("fakeBackend: no interactions %s/%s", sessionID, file)
	}
	return ifile, nil
}

func (f *fakeBackend) SaveInteractions(sessionID, file string, ifile InteractionFile) error {
	f.files[f.key(sessionID, file)] = ifile
	return nil
}

func TestService_Load(t *testing.T) {
	backend := newFakeBackend()
	backend.files["s1/developer"] = InteractionFile{Agent: "developer", Questions: []Question{{ID: "q1", Question: "Which approach?"}}}
	svc := NewService(backend)

	f, err := svc.Load("s1", "developer")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.Questions) != 1 {
		t.Fatalf("expected 1 question, got %+v", f)
	}
}

func TestService_Load_MissingFile(t *testing.T) {
	svc := NewService(newFakeBackend())
	if _, err := svc.Load("s1", "missing"); err == nil {
		t.Fatalf("expected error for a missing interactions file")
	}
}

func TestService_SubmitAnswers_MergesByQuestionID(t *testing.T) {
	backend := newFakeBackend()
	backend.files["s1/developer"] = InteractionFile{
		Agent: "developer",
		Questions: []Question{
			{ID: "q1", Question: "Which approach?", Options: []string{"A", "B"}},
			{ID: "q2", Question: "Need a migration?"},
		},
	}
	svc := NewService(backend)

	updated, err := svc.SubmitAnswers("s1", "developer", map[string]string{
		"q1": "A",
		"q2": "Yes, add one",
	})
	if err != nil {
		t.Fatalf("SubmitAnswers: %v", err)
	}
	if updated.Questions[0].Answer == nil || *updated.Questions[0].Answer != "A" {
		t.Fatalf("expected q1 answered 'A', got %+v", updated.Questions[0])
	}
	if updated.Questions[1].Answer == nil || *updated.Questions[1].Answer != "Yes, add one" {
		t.Fatalf("expected q2 answered, got %+v", updated.Questions[1])
	}

	persisted := backend.files["s1/developer"]
	if persisted.Questions[0].Answer == nil || *persisted.Questions[0].Answer != "A" {
		t.Fatalf("expected persisted file to reflect the answer, got %+v", persisted)
	}
}

func TestService_SubmitAnswers_PartialAnswersLeaveOthersUntouched(t *testing.T) {
	backend := newFakeBackend()
	existing := "already answered"
	backend.files["s1/developer"] = InteractionFile{
		Agent: "developer",
		Questions: []Question{
			{ID: "q1", Question: "First?", Answer: &existing},
			{ID: "q2", Question: "Second?"},
		},
	}
	svc := NewService(backend)

	updated, err := svc.SubmitAnswers("s1", "developer", map[string]string{"q2": "now answered"})
	if err != nil {
		t.Fatalf("SubmitAnswers: %v", err)
	}
	if updated.Questions[0].Answer == nil || *updated.Questions[0].Answer != "already answered" {
		t.Fatalf("expected q1's existing answer to survive untouched, got %+v", updated.Questions[0])
	}
	if updated.Questions[1].Answer == nil || *updated.Questions[1].Answer != "now answered" {
		t.Fatalf("expected q2 answered, got %+v", updated.Questions[1])
	}
}

func TestService_SubmitAnswers_UnknownQuestionIDIgnored(t *testing.T) {
	backend := newFakeBackend()
	backend.files["s1/developer"] = InteractionFile{
		Agent:     "developer",
		Questions: []Question{{ID: "q1", Question: "Only one?"}},
	}
	svc := NewService(backend)

	updated, err := svc.SubmitAnswers("s1", "developer", map[string]string{"ghost-id": "ignored"})
	if err != nil {
		t.Fatalf("SubmitAnswers: %v", err)
	}
	if updated.Questions[0].Answer != nil {
		t.Fatalf("expected q1 to remain unanswered, got %+v", updated.Questions[0])
	}
}

func TestAllAnswered(t *testing.T) {
	answer := "x"
	tests := []struct {
		name string
		file InteractionFile
		want bool
	}{
		{"no questions", InteractionFile{}, true},
		{"all answered", InteractionFile{Questions: []Question{{ID: "q1", Answer: &answer}, {ID: "q2", Answer: &answer}}}, true},
		{"one unanswered", InteractionFile{Questions: []Question{{ID: "q1", Answer: &answer}, {ID: "q2"}}}, false},
	}
	for _, tt := range tests {
		if got := AllAnswered(tt.file); got != tt.want {
			t.Fatalf("%s: AllAnswered() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestResumePrompt(t *testing.T) {
	got := ResumePrompt("developer", "developer-2")
	want := "Your questions have been answered. Read interactions/developer-2.json and continue."
	if got != want {
		t.Fatalf("ResumePrompt() = %q, want %q", got, want)
	}
}
