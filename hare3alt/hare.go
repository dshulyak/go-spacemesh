package hare

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/timesync"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const protocolName = "/h3"

type Config struct {
	Committee       int           `mapstructure:"committee"`
	Leaders         int           `mapstructure:"leaders"` // leaders are relevant only proposal round
	IterationsLimit uint8         `mapstructure:"iterations-limit"`
	PreroundDelay   time.Duration `mapstructure:"preround-delay"` // hare starts preround after this delay
	RoundDuration   time.Duration `mapstructure:"round-duration"`
}

type ConsensusOutput struct {
	Layer     types.LayerID
	Proposals []types.ProposalID
}

type WeakCoinOutput struct {
	Layer types.LayerID
	Coin  bool
}

type instanceInput struct {
	message message
	result  chan error
}

type instanceInputs struct {
	ctx    context.Context
	inputs chan<- *instanceInput
}

func (inputs *instanceInputs) submit(msg message) error {
	input := &instanceInput{
		message: msg,
		result:  make(chan error, 1),
	}
	select {
	case <-inputs.ctx.Done():
		return inputs.ctx.Err()
	case inputs.inputs <- input:
	}
	select {
	case err := <-input.result:
		return err
	case <-inputs.ctx.Done():
		return inputs.ctx.Err()
	}
}

type Hare struct {
	ctx    context.Context
	cancel context.CancelFunc
	eg     errgroup.Group

	results chan ConsensusOutput
	coins   chan WeakCoinOutput

	config Config

	log       *zap.Logger
	wallclock clock.Clock
	nodeclock *timesync.NodeClock
	publisher pubsub.Publisher
	db        *datastore.CachedDB
	signer    *signing.EdSigner
	sync      system.SyncStateProvider
	beacon    system.BeaconGetter

	mu        sync.Mutex
	instances map[types.LayerID]instanceInputs
}

func (h *Hare) Results() <-chan ConsensusOutput {
	return h.results
}

func (h *Hare) Coins() <-chan WeakCoinOutput {
	return h.coins
}

func (h *Hare) Start() {
	h.eg.Go(func() error {
		for next := h.nodeclock.CurrentLayer() + 1; ; next++ {
			select {
			case <-h.nodeclock.AwaitLayer(next):
				h.onLayer(next)
			case <-h.ctx.Done():
				return nil
			}
		}
	})
}

func (h *Hare) onLayer(lid types.LayerID) {
	if !h.sync.IsSynced(h.ctx) {
		return
	}
	beacon, err := h.beacon.GetBeacon(lid.GetEpoch())
	if err != nil {
		return
	}
	h.mu.Lock()
	inputs := make(chan *instanceInput, 32)
	ctx, cancel := context.WithCancel(h.ctx)
	h.instances[lid] = instanceInputs{
		ctx:    ctx,
		inputs: inputs,
	}
	h.mu.Unlock()
	h.eg.Go(func() error {
		if err := h.run(lid, beacon, inputs); err != nil {
			h.log.Warn("hare failed",
				zap.Uint32("lid", lid.Uint32()),
				zap.Error(err),
			)
		} else {
			h.log.Debug("hare terminated",
				zap.Uint32("lid", lid.Uint32()),
			)
		}
		cancel()
		h.mu.Lock()
		delete(h.instances, lid)
		h.mu.Unlock()
		return nil
	})
}

func (h *Hare) run(layer types.LayerID, beacon types.Beacon, inputs <-chan *instanceInput) error {
	walltime := h.nodeclock.LayerToTime(layer)
	select {
	case <-h.wallclock.After(h.wallclock.Until(walltime.Add(h.config.PreroundDelay))):
	case <-h.ctx.Done():
	}
	walltime = walltime.Add(h.config.PreroundDelay)
	proposals, err := h.proposals(layer, beacon)
	if err != nil {
		return err
	}
	proto := protocol{
		iteration: 0,
		round:     preround,
		layer:     layer,
		initial:   proposals,
	}
	if msg := proto.active(); msg != nil {
		h.emitMsg(msg)
	}
	if err := h.onOutput(layer, proto.next()); err != nil {
		return err
	}
	for {
		select {
		case input := <-inputs:
			h.log.Debug("received message", zap.Inline(&input.message))
			input.result <- proto.onMessage(&input.message)
		case <-h.wallclock.After(h.wallclock.Until(walltime.Add(h.config.RoundDuration))):
			if msg := proto.active(); msg != nil {
				h.emitMsg(msg)
			}
			out := proto.next()
			if err := h.onOutput(layer, out); err != nil {
				return err
			}
			if out.terminated {
				return nil
			}
			if proto.iteration == h.config.IterationsLimit {
				return fmt.Errorf("hare didn't reach consensus in %d iterations", h.config.IterationsLimit)
			}
			walltime = walltime.Add(h.config.RoundDuration)
		case <-h.ctx.Done():
			return nil
		}
	}
}

func (h *Hare) emitMsg(msg *message) {
	h.log.Debug("emit message", zap.Inline(msg))
}

func (h *Hare) onOutput(layer types.LayerID, out output) error {
	h.log.Debug("output", zap.Uint32("lid", layer.Uint32()), zap.Inline(&out))
	if out.coin != nil {
		select {
		case h.coins <- WeakCoinOutput{Layer: layer, Coin: *out.coin}:
		default:
			return fmt.Errorf("coins channel is expected to be drained")
		}
	}
	if out.result != nil {
		select {
		case h.results <- ConsensusOutput{Layer: layer, Proposals: out.result}:
		default:
			return fmt.Errorf("results channel is expected to be drained")
		}
	}
	return nil
}

func (h *Hare) proposals(lid types.LayerID, beacon types.Beacon) ([]types.ProposalID, error) {
	return nil, nil
}

func (h *Hare) Stop() {
	h.cancel()
	h.eg.Wait()
	select {
	case <-h.results: // to prevent panic on second stop
		return
	default:
	}
	close(h.results)
	close(h.coins)
}
