// Package idgen provides the one real IDGenerator implementation domain
// services use for new session IDs.
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
