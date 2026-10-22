package idgen

import (
	"github.com/google/uuid"

	"github.com/MayurWinjit/monday/internal/domain/session"
)

var _ session.IDGenerator = UUID{}

type UUID struct{}

func (UUID) NewID() string {
	return uuid.NewString()
}
