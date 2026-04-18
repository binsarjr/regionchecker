// Package clock exposes a Clock interface so tests can inject deterministic time.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// Real returns a Clock backed by the system clock.
func Real() Clock {
	return realClock{}
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Fixed returns a Clock that always reports t.
func Fixed(t time.Time) Clock {
	return fixed{t}
}

type fixed struct{ t time.Time }

func (f fixed) Now() time.Time { return f.t }
