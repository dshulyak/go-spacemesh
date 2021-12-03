package sim

import (
	"testing"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

// StateMachine ...
type StateMachine interface {
	OnEvent(Event) []Event
}

// Coordinator ...
type Coordinator struct {
	from, to types.LayerID

	instances []StateMachine
}

// Run simulation.
func (r *Coordinator) Run(tb testing.TB) {
	for lid := r.from; !lid.After(r.to); lid = lid.Add(1) {

	}
}
