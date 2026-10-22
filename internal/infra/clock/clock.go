// Package clock provides the one real implementation of the Clock port
// domain services use for timestamps, so production code never calls
// time.Now() directly and tests never need a real wall clock.
package clock

import (
	"time"

	"github.com/MayurWinjit/monday/internal/domain/pipeline"
	"github.com/MayurWinjit/monday/internal/domain/session"
)

var (
	_ pipeline.Clock = System{}
	_ session.Clock  = System{}
)

type System struct{}

func (System) Now() time.Time {
	return time.Now()
}
