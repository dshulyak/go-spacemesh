package hare3alt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/timesync"
)

const protocolName = "/h3"

type Config struct {
	Committee       uint16        `mapstructure:"committee"`
	Leaders         int           `mapstructure:"leaders"`
	IterationsLimit uint8         `mapstructure:"iterations-limit"`
	PreroundDelay   time.Duration `mapstructure:"preround-delay"`
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
	message *message
	result  chan *messageResponse
}

type messageResponse struct {
	gossip       bool
	equivocation *types.HareProof
}

type instanceInputs struct {
	ctx    context.Context
	inputs chan<- *instanceInput
}

func (inputs *instanceInputs) submit(msg *message) (*messageResponse, error) {
	input := &instanceInput{
		message: msg,
		result:  make(chan *messageResponse, 1),
	}
	select {
	case <-inputs.ctx.Done():
		return nil, inputs.ctx.Err()
	case inputs.inputs <- input:
	}
	select {
	case resp := <-input.result:
		return resp, nil
	case <-inputs.ctx.Done():
		return nil, inputs.ctx.Err()
	}
}

type Hare struct {
	ctx     context.Context
	cancel  context.CancelFunc
	eg      errgroup.Group
	results chan ConsensusOutput
	coins   chan WeakCoinOutput

	config    Config
	log       *zap.Logger
	wallclock clock.Clock

	nodeclock *timesync.NodeClock
	pubsub    pubsub.PublishSubsciber
	db        *datastore.CachedDB
	signer    *signing.EdSigner
	oracle    *LegacyOracle
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

func (h *Hare) handler(ctx context.Context, peer p2p.Peer, buf []byte) error {
	msg, err := decodeMessage(buf)
	if err != nil {
		return err
	}
	h.mu.Lock()
	instance, registered := h.instances[msg.layer]
	h.mu.Unlock()
	if !registered {
		return fmt.Errorf("layer %d not registered", msg.layer)
	}

	// check signature and store message hash
	if err := h.oracle.validate(msg); err != nil {
		return err
	}
	if malicious, err := h.db.IsMalicious(msg.sender); err != nil {
		return fmt.Errorf("database error %s", err.Error())
	} else {
		msg.malicious = malicious
	}
	resp, err := instance.submit(msg)
	if err != nil {
		return err
	}
	if resp.equivocation != nil {
		// TODO(dshulyak) save and gossip equivocation
		_ = resp.equivocation
	}
	if !resp.gossip {
		return fmt.Errorf("dropped by graded gossip")
	}
	return nil
}

func (h *Hare) Start() {
	h.eg.Go(func() error {
		h.pubsub.Register(protocolName, h.handler)
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

func (h *Hare) onLayer(layer types.LayerID) {
	if !h.sync.IsSynced(h.ctx) {
		return
	}
	beacon, err := h.beacon.GetBeacon(layer.GetEpoch())
	if err != nil || beacon == types.EmptyBeacon {
		return
	}
	h.mu.Lock()
	inputs := make(chan *instanceInput, 32)
	ctx, cancel := context.WithCancel(h.ctx)
	h.instances[layer] = instanceInputs{
		ctx:    ctx,
		inputs: inputs,
	}
	h.mu.Unlock()
	h.eg.Go(func() error {
		if err := h.run(layer, beacon, inputs); err != nil {
			h.log.Warn("hare failed",
				zap.Uint32("lid", layer.Uint32()),
				zap.Error(err),
			)
		} else {
			h.log.Debug("hare terminated",
				zap.Uint32("lid", layer.Uint32()),
			)
		}
		cancel()
		h.mu.Lock()
		delete(h.instances, layer)
		h.mu.Unlock()
		return nil
	})
}

func (h *Hare) run(layer types.LayerID, beacon types.Beacon, inputs <-chan *instanceInput) error {
	// implementation needs to load a lot of data from disk
	// we do it before preround starts, so that it can have some slack time
	vrf := h.oracle.active(h.signer.NodeID(), layer, iterround{round: preround})
	walltime := h.nodeclock.LayerToTime(layer).Add(h.config.PreroundDelay)
	select {
	case <-h.wallclock.After(h.wallclock.Until(walltime)):
	case <-h.ctx.Done():
	}
	proposals, err := h.proposals(layer, beacon)
	if err != nil {
		return err
	}
	proto := newProtocol(proposals, h.config.Committee/2+1)
	if err := h.onOutput(layer, proto.next(vrf != nil), vrf); err != nil {
		return err
	}
	walltime = walltime.Add(h.config.RoundDuration)
	for {
		select {
		case input := <-inputs:
			gossip, equivocation := proto.onMessage(input.message)
			h.log.Debug("on message",
				zap.Inline(input.message),
				zap.Bool("gossip", gossip),
			)
			input.result <- &messageResponse{gossip: gossip, equivocation: equivocation}
		case <-h.wallclock.After(h.wallclock.Until(walltime)):
			vrf := h.oracle.active(h.signer.NodeID(), layer, proto.iterround)
			out := proto.next(vrf != nil)
			if err := h.onOutput(layer, out, vrf); err != nil {
				return err
			}
			if out.terminated {
				return nil
			}
			if proto.iter == h.config.IterationsLimit {
				return fmt.Errorf("hare failed to reach consensus in %d iterations", h.config.IterationsLimit)
			}
			walltime = walltime.Add(h.config.RoundDuration)
		case <-h.ctx.Done():
			return nil
		}
	}
}

func (h *Hare) gossipMessage(msg *message) {}

func (h *Hare) onOutput(layer types.LayerID, out output, vrf *proof) error {
	h.log.Debug("round output",
		zap.Uint32("lid", layer.Uint32()),
		zap.Inline(&out),
		zap.Bool("active", vrf != nil),
	)
	if out.message != nil {
		if vrf == nil {
			panic("inconsistent state")
		}
		out.message.eligibilities = vrf.eligibilities
		out.message.vrf = vrf.vrf
		h.gossipMessage(out.message)
	}
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
