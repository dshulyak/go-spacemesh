package sim

import (
	"github.com/spacemeshos/go-spacemesh/activation"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/spacemeshos/go-spacemesh/signing"
)

// StateMachine ...
type StateMachine interface {
	// OnEvent event.
	OnEvent(Event) []Event
}

var _ StateMachine = (*Core)(nil)

// Core state machine. Represents single instance of the tortoise consensus.
type Core struct {
	meshdb   *mesh.DB
	atxdb    *activation.DB
	beacons  interface{}
	tortoise interface{}

	key signing.Signer
}

// OnEvent blocks, atx, input vector, beacon, coinflip and store them.
// Generate atx at the end of each epoch.
// Generate block at the start of every layer.
func (c *Core) OnEvent(event Event) []Event {
	return nil
}
