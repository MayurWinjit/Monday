package interaction

import "fmt"

// Backend is the shape Service needs from the filesystem layer. Defined
// locally (not imported from the root domain package) for the same
// import-cycle-avoidance reason as pipeline.Backend / session.Backend -
// internal/infra/fsstore.Store satisfies this structurally.
type Backend interface {
	LoadInteractions(sessionID, file string) (InteractionFile, error)
	SaveInteractions(sessionID, file string, f InteractionFile) error
}

// Service is the interactions use case from architect-plan section 6: load
// a pending questions file, merge submitted answers back into it, and
// produce the fixed resume-prompt template session.Service sends once every
// pending question has been answered.
type Service struct {
	backend Backend
}

func NewService(backend Backend) *Service {
	return &Service{backend: backend}
}

func (s *Service) Load(sessionID, file string) (InteractionFile, error) {
	f, err := s.backend.LoadInteractions(sessionID, file)
	if err != nil {
		return InteractionFile{}, fmt.Errorf("interaction: load %s/%s: %w", sessionID, file, err)
	}
	return f, nil
}

// SubmitAnswers merges each answer into the matching Question.Answer (by
// question ID) and writes the file back - write-temp-then-rename is
// Backend's responsibility (infra/fsstore), not this service's. Answers for
// question IDs not present in the file are ignored rather than erroring,
// since the UI only ever submits answers for questions it rendered from
// this exact file.
func (s *Service) SubmitAnswers(sessionID, file string, answers map[string]string) (InteractionFile, error) {
	f, err := s.backend.LoadInteractions(sessionID, file)
	if err != nil {
		return InteractionFile{}, fmt.Errorf("interaction: submit answers: load %s/%s: %w", sessionID, file, err)
	}

	for i := range f.Questions {
		if answer, ok := answers[f.Questions[i].ID]; ok {
			a := answer
			f.Questions[i].Answer = &a
		}
	}

	if err := s.backend.SaveInteractions(sessionID, file, f); err != nil {
		return InteractionFile{}, fmt.Errorf("interaction: submit answers: save %s/%s: %w", sessionID, file, err)
	}
	return f, nil
}

// AllAnswered reports whether every question in f has a non-nil Answer.
func AllAnswered(f InteractionFile) bool {
	for _, q := range f.Questions {
		if q.Answer == nil {
			return false
		}
	}
	return true
}

// ResumePrompt is the fixed template session.Service sends (via
// PromptAgent+SendKeys) once every pending question in file has been
// answered. agentName is accepted for signature parity with
// architect-plan section 6 and future personalization, though the current
// fixed template does not need it - the file name alone is enough for the
// agent to know what to re-read.
func ResumePrompt(agentName, file string) string {
	return fmt.Sprintf("Your questions have been answered. Read interactions/%s.json and continue.", file)
}
