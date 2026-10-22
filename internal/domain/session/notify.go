package session

import (
	"fmt"

	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
)

// NotifyRouter is the single dispatch point from a decoded NotifyMessage
// (as delivered by infra/socket's event channel) to the matching Service
// method. Both a CLI harness and the TUI's progress screen route every
// incoming notify event through the same two Service methods - Route
// exists so neither caller has to duplicate the type switch.
type NotifyRouter struct {
	svc *Service
}

func NewNotifyRouter(svc *Service) *NotifyRouter {
	return &NotifyRouter{svc: svc}
}

// Route dispatches msg to HandleDone or HandleQuestion, returning the
// updated Session plus - for a QUES message only - its pending
// InteractionFile.
func (r *NotifyRouter) Route(msg notifyproto.NotifyMessage) (Session, *interaction.InteractionFile, error) {
	switch msg.Type {
	case notifyproto.NotifyDone:
		sess, err := r.svc.HandleDone(msg)
		return sess, nil, err
	case notifyproto.NotifyQuestion:
		sess, file, err := r.svc.HandleQuestion(msg)
		if err != nil {
			return Session{}, nil, err
		}
		return sess, &file, nil
	default:
		return Session{}, nil, fmt.Errorf("session: notify router: unknown notify type %q", msg.Type)
	}
}
