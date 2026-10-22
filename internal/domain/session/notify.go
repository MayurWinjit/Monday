package session

import (
	"fmt"

	"github.com/MayurWinjit/monday/internal/domain/interaction"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
)

type NotifyRouter struct {
	svc *Service
}

func NewNotifyRouter(svc *Service) *NotifyRouter {
	return &NotifyRouter{svc: svc}
}

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
