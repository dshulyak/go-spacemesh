package sim

import (
	"math/rand"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

var _ StateMachine = (*hare)(nil)

func newHare(rng *rand.Rand) *hare {
	return &hare{rng: rng}
}

// hare is an instance of the hare consensus.
// At the end of each layer it outputs input vector and coinflip events.
type hare struct {
	rng  *rand.Rand
	bids map[types.BlockID]struct{}
}

// OnEvent blocks and produce layer vector at the end of layer.
func (h *hare) OnEvent(event Event) []Event {
	switch ev := event.(type) {
	case EventLayerStart:
		h.bids = map[types.BlockID]struct{}{}
	case EventHareStarted:
		var bids []types.BlockID
		for bid := range h.bids {
			bids = append(bids, bid)
		}
		// head and tails are at equal probability.
		return []Event{
			EventCoinflip{LayerID: ev.LayerID, Coinflip: h.rng.Int()%2 == 0},
			EventLayerVector{LayerID: ev.LayerID, Vector: bids},
		}
	case EventBlock:
		h.bids[ev.Block.ID()] = struct{}{}
	}
	return nil
}
