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
