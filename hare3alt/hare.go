package hare3alt

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/proposals"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/timesync"
)

const protocolName = "/h3"

type Config struct {
	Enable          bool          `mapstructure:"enable"`
	EnableAfter     types.LayerID `mapstructure:"enable-layer"`
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

	nodeclock  *timesync.NodeClock
	pubsub     pubsub.PublishSubsciber
	db         *datastore.CachedDB
	edVerifier *signing.EdVerifier
	signer     *signing.EdSigner
	oracle     *LegacyOracle
	sync       system.SyncStateProvider
	beacon     system.BeaconGetter

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
	if !h.edVerifier.Verify(signing.HARE, msg.sender, msg.signedBytes(), msg.signature) {
		return fmt.Errorf("%w: invalid signature", pubsub.ErrValidationReject)
	}
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
		enabled := types.MaxLayer(h.nodeclock.CurrentLayer(), h.config.EnableAfter)
		for next := enabled + 1; ; next++ {
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
	// oracle may load non-negligible amount of data from disk
	// we do it before preround starts, so that load can have some slack time
	// before it needs to be used in validation
	vrf := h.oracle.active(h.signer.NodeID(), layer, iterround{round: preround})
	walltime := h.nodeclock.LayerToTime(layer).Add(h.config.PreroundDelay)
	select {
	case <-h.wallclock.After(h.wallclock.Until(walltime)):
	case <-h.ctx.Done():
	}
	var proposals []types.ProposalID
	// initial set doesn't matter if node is not active in preround
	if vrf != nil {
		proposals = h.proposals(layer, beacon)
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
				return fmt.Errorf("hare failed to reach consensus in %d iterations",
					h.config.IterationsLimit)
			}
			walltime = walltime.Add(h.config.RoundDuration)
		case <-h.ctx.Done():
			return nil
		}
	}
}

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
		if err := h.pubsub.Publish(
			h.ctx, protocolName,
			encodeWithSignature(out.message, h.signer)); err != nil {
			h.log.Error("failed to publish", zap.Inline(out.message))
		}
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

func (h *Hare) proposals(lid types.LayerID, epochBeacon types.Beacon) []types.ProposalID {
	props, err := proposals.GetByLayer(h.db, lid)
	if err != nil {
		if errors.Is(err, sql.ErrNotFound) {
			h.log.Warn("no proposals found for hare, using empty set",
				zap.Uint32("lid", lid.Uint32()), zap.Error(err))
		} else {
			h.log.Error("failed to get proposals for hare",
				zap.Uint32("lid", lid.Uint32()), zap.Error(err))
		}
		return []types.ProposalID{}
	}

	var (
		beacon        types.Beacon
		result        []types.ProposalID
		ownHdr        *types.ActivationTxHeader
		ownTickHeight = uint64(math.MaxUint64)
	)

	ownHdr, err = h.db.GetEpochAtx(lid.GetEpoch()-1, h.signer.NodeID())
	if err != nil && !errors.Is(err, sql.ErrNotFound) {
		return []types.ProposalID{}
	}
	if ownHdr != nil {
		ownTickHeight = ownHdr.TickHeight()
	}
	atxs := map[types.ATXID]int{}
	for _, p := range props {
		atxs[p.AtxID]++
	}
	for _, p := range props {
		if p.IsMalicious() {
			h.log.Warn("not voting on proposal from malicious identity",
				zap.Stringer("id", p.ID()),
			)
			continue
		}
		if n := atxs[p.AtxID]; n > 1 {
			h.log.Warn("proposal with same atx added several times in the recorded set",
				zap.Int("n", n),
				zap.Stringer("id", p.ID()),
				zap.Stringer("atxid", p.AtxID),
			)
			continue
		}
		if ownHdr != nil {
			hdr, err := h.db.GetAtxHeader(p.AtxID)
			if err != nil {
				return []types.ProposalID{}
			}
			if hdr.BaseTickHeight >= ownTickHeight {
				// does not vote for future proposal
				h.log.Warn("proposal base tick height too high. skipping",
					zap.Uint32("lid", lid.Uint32()),
					zap.Uint64("proposal_height", hdr.BaseTickHeight),
					zap.Uint64("own_height", ownTickHeight),
				)
				continue
			}
		}
		if p.EpochData != nil {
			beacon = p.EpochData.Beacon
		} else if p.RefBallot == types.EmptyBallotID {
			return []types.ProposalID{}
		} else if refBallot, err := ballots.Get(h.db, p.RefBallot); err != nil {
			return []types.ProposalID{}
		} else if refBallot.EpochData == nil {
			return []types.ProposalID{}
		} else {
			beacon = refBallot.EpochData.Beacon
		}

		if beacon == epochBeacon {
			result = append(result, p.ID())
		} else {
			h.log.Warn("proposal has different beacon value",
				zap.Uint32("lid", lid.Uint32()),
				zap.Stringer("id", p.ID()),
				zap.String("proposal_beacon", beacon.ShortString()),
				zap.String("epoch_beacon", epochBeacon.ShortString()),
			)
		}
	}
	return result
}

func (h *Hare) Stop() {
	h.cancel()
	h.eg.Wait()
	close(h.results)
	close(h.coins)
}
