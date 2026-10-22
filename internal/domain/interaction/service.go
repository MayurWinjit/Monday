package interaction

import "fmt"

type Backend interface {
	LoadInteractions(sessionID, file string) (InteractionFile, error)
	SaveInteractions(sessionID, file string, f InteractionFile) error
}

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

func AllAnswered(f InteractionFile) bool {
	for _, q := range f.Questions {
		if q.Answer == nil {
			return false
		}
	}
	return true
}

func ResumePrompt(agentName, file string) string {
	return fmt.Sprintf("Your questions have been answered. Read interactions/%s.json and continue.", file)
}
