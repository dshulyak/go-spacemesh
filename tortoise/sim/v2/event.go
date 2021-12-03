package sim

import "github.com/spacemeshos/go-spacemesh/common/types"

// Event is an alias to interface.
type Event interface{}

// EventBlock is an event producing a block.
type EventBlock struct {
	Block *types.Block
}

// EventAtx is an event producing an atx.
type EventAtx struct {
	Atx *types.ActivationTx
}

type (
	// Layer clock events.

	// EventLayerStart ...
	EventLayerStart struct {
		LayerID types.LayerID
	}

	// EventLayerEnd ...
	EventLayerEnd struct {
		LayerID types.LayerID
	}
)

// EventHareStarted is sent after EventLayerStart.
type EventHareStarted struct {
	LayerID types.LayerID
}

// EventHareTerminated concludes layer, at this point tortoise needs to be executed.
// If reordered - later layer wil be terminated earlier.
type EventHareTerminated struct {
	LayerID types.LayerID
}

// EventLayerVector is an event producing a layer vector.
type EventLayerVector struct {
	LayerID types.LayerID
	Vector  []types.BlockID
}

// EventCoinflip is an event producing coinflip.
type EventCoinflip struct {
	LayerID  types.LayerID
	Coinflip bool
}

// EventBeacon is an event producing a beacon for epoch.
// This event can be raised during epoch:
// 0 - as if beacon failed
// 1 - there is only one consensus on beacon
// N - multiple instances of consensus of beacon.
type EventBeacon struct {
	EpochID types.EpochID
	Beacon  []byte
}
