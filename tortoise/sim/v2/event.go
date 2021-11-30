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
type EventBeacon struct {
	EpochID types.EpochID
	Beacon  []byte
}

// EventLayerStart ...
type EventLayerStart struct {
	LayerID types.LayerID
}

// EventLayerEnd ...
type EventLayerEnd struct {
	LayerID types.LayerID
}
