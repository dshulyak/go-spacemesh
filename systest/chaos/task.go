package chaos

import (
	"time"

	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

type Selector func(pod string) (bool, error)

type Chaos interface {
	Apply(ctx *testcontext.Context, name string, pods ...string) (Teardown, error)
}

type Task struct {
	Name     string
	Timed    *Timed
	Selector Selector
	Chaos    Chaos
}

type Timed struct {
	// Duration controls how long Chaos will be applied.
	Duration time.Duration
	// Chaos is applied periodically +- random time within jitter.
	Period, Jitter time.Duration
}
