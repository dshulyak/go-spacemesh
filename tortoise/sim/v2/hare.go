package sim

import "github.com/spacemeshos/go-spacemesh/common/types"

var _ StateMachine = (*Hare)(nil)

// Hare is an instance of the hare consensus.
// For each layer it outputs input vector and coinflip events.
type Hare struct {
	lid      types.LayerID
	prevCoin bool
	bids     map[types.BlockID]struct{}
}

// OnEvent blocks and produce layer vector at the end of layer.
func (h *Hare) OnEvent(event Event) []Event {
	switch ev := event.(type) {
	case EventLayerStart:
		h.lid = ev.LayerID
		h.bids = map[types.BlockID]struct{}{}
	case EventLayerEnd:
		var bids []types.BlockID
		for bid := range h.bids {
			bids = append(bids, bid)
		}
		// head and tails are at equal probability.
		coin := !h.prevCoin
		h.prevCoin = coin
		return []Event{
			EventCoinflip{Coinflip: coin},
			EventLayerVector{LayerID: h.lid, Vector: bids},
		}
	case EventBlock:
		h.bids[ev.Block.ID()] = struct{}{}
	}
	return nil
}
