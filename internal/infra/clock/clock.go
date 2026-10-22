// Package clock provides the one real implementation of the Clock port
// domain services use for timestamps, so production code never calls
// time.Now() directly and tests never need a real wall clock.
package clock

import "time"

type System struct{}

func (System) Now() time.Time {
	return time.Now()
}
