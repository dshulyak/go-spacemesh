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

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/beacons"
	"github.com/spacemeshos/go-spacemesh/sql/proposals"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/timesync"
)

type Config struct {
	Enable          bool          `mapstructure:"enable"`
	EnableAfter     types.LayerID `mapstructure:"enable-layer"`
	Committee       uint16        `mapstructure:"committee"`
	Leaders         int           `mapstructure:"leaders"`
	IterationsLimit uint8         `mapstructure:"iterations-limit"`
	PreroundDelay   time.Duration `mapstructure:"preround-delay"`
	RoundDuration   time.Duration `mapstructure:"round-duration"`
	ProtocolName    string
}

func DefaultConfig() Config {
	return Config{
		Enable:          true,
		Committee:       800,
		Leaders:         10,
		IterationsLimit: 40,
		PreroundDelay:   25 * time.Second,
		RoundDuration:   10 * time.Second,
		ProtocolName:    "/h/3.0", // can be bumped to 3.1 with oracle upgrade
	}
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
	input  *input
	result chan *response
}

type response struct {
	gossip       bool
	equivocation *types.HareProof
}

type instanceInputs struct {
	ctx    context.Context
	inputs chan<- *instanceInput
}

func (inputs *instanceInputs) submit(msg *input) (*response, error) {
	input := &instanceInput{
		input:  msg,
		result: make(chan *response, 1),
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

type Opt func(*Hare)

func WithWallclock(clock clock.Clock) Opt {
	return func(hr *Hare) {
		hr.wallclock = clock
	}
}

func WithConfig(cfg Config) Opt {
	return func(hr *Hare) {
		hr.config = cfg
		hr.oracle.config = cfg
	}
}

func WithLogger(logger *zap.Logger) Opt {
	return func(hr *Hare) {
		hr.log = logger
		hr.oracle.log = logger
	}
}

// WithEnableLayer can be used to pass genesis layer.
// Note that for it to have effect it needs to be after WithConfig.
func WithEnableLayer(layer types.LayerID) Opt {
	return func(hr *Hare) {
		hr.config.EnableAfter = layer
		hr.oracle.config.EnableAfter = layer
	}
}

func New(
	nodeclock *timesync.NodeClock,
	pubsub pubsub.PublishSubsciber,
	db *datastore.CachedDB,
	verifier *signing.EdVerifier,
	signer *signing.EdSigner,
	oracle oracle,
	sync system.SyncStateProvider,
	opts ...Opt,
) *Hare {
	ctx, cancel := context.WithCancel(context.Background())
	hr := &Hare{
		ctx:       ctx,
		cancel:    cancel,
		results:   make(chan ConsensusOutput, 32),
		coins:     make(chan WeakCoinOutput, 32),
		instances: map[types.LayerID]instanceInputs{},

		config:    DefaultConfig(),
		log:       zap.NewNop(),
		wallclock: clock.New(),

		nodeclock: nodeclock,
		pubsub:    pubsub,
		db:        db,
		verifier:  verifier,
		signer:    signer,
		oracle: &legacyOracle{
			log:    zap.NewNop(),
			oracle: oracle,
			config: DefaultConfig(),
		},
		sync: sync,
	}
	for _, opt := range opts {
		opt(hr)
	}
	return hr
}

type Hare struct {
	// state
	ctx       context.Context
	cancel    context.CancelFunc
	eg        errgroup.Group
	results   chan ConsensusOutput
	coins     chan WeakCoinOutput
	mu        sync.Mutex
	instances map[types.LayerID]instanceInputs

	// options
	config    Config
	log       *zap.Logger
	wallclock clock.Clock

	// dependencies
	nodeclock *timesync.NodeClock
	pubsub    pubsub.PublishSubsciber
	db        *datastore.CachedDB
	verifier  *signing.EdVerifier
	signer    *signing.EdSigner
	oracle    *legacyOracle
	sync      system.SyncStateProvider
}

func (h *Hare) Results() <-chan ConsensusOutput {
	return h.results
}

func (h *Hare) Coins() <-chan WeakCoinOutput {
	return h.coins
}

func (h *Hare) Start() {
	h.eg.Go(func() error {
		h.pubsub.Register(h.config.ProtocolName, h.handler)
		enabled := types.MaxLayer(h.nodeclock.CurrentLayer(), h.config.EnableAfter)
		h.log.Debug("starting at layer", zap.Uint32("lid", enabled.Uint32()))
		for next := enabled + 1; ; next++ {
			select {
			case <-h.nodeclock.AwaitLayer(next):
				h.log.Debug("notified", zap.Uint32("lid", next.Uint32()))
				h.onLayer(next)
			case <-h.ctx.Done():
				return nil
			}
		}
	})
}

func (h *Hare) Running() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.instances)
}

func (h *Hare) handler(ctx context.Context, peer p2p.Peer, buf []byte) error {
	msg := &Message{}
	if err := codec.Decode(buf, msg); err != nil {
		return fmt.Errorf("%w: decoding error %s", pubsub.ErrValidationReject, err.Error())
	}
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("%w: validation %s", pubsub.ErrValidationReject, err.Error())
	}
	h.mu.Lock()
	instance, registered := h.instances[msg.Layer]
	h.mu.Unlock()
	if !registered {
		return fmt.Errorf("layer %d is not registered", msg.Layer)
	}
	if !h.verifier.Verify(signing.HARE, msg.Sender, msg.ToMetadata().ToBytes(), msg.Signature) {
		return fmt.Errorf("%w: invalid signature", pubsub.ErrValidationReject)
	}
	malicious, err := h.db.IsMalicious(msg.Sender)
	if err != nil {
		return fmt.Errorf("database error %s", err.Error())
	}
	g := h.oracle.validate(msg)
	if g == grade0 {
		return fmt.Errorf("zero grade")
	}
	resp, err := instance.submit(&input{
		Message:   msg,
		msgHash:   msg.ToHash(),
		malicious: malicious,
		grade:     g,
	})
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

func (h *Hare) onLayer(layer types.LayerID) {
	if !h.sync.IsSynced(h.ctx) {
		h.log.Debug("not synced", zap.Uint32("lid", layer.Uint32()))
		return
	}
	beacon, err := beacons.Get(h.db, layer.GetEpoch())
	if err != nil || beacon == types.EmptyBeacon {
		h.log.Debug("no beacon",
			zap.Uint32("epoch", layer.GetEpoch().Uint32()),
			zap.Uint32("lid", layer.Uint32()),
			zap.Error(err),
		)
		return
	}
	inputs := make(chan *instanceInput)
	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.instances[layer] = instanceInputs{
		ctx:    ctx,
		inputs: inputs,
	}
	h.mu.Unlock()
	h.log.Debug("registered layer", zap.Uint32("lid", layer.Uint32()))
	h.eg.Go(func() error {
		if err := h.run(layer, beacon, inputs); err != nil {
			h.log.Warn("failed",
				zap.Uint32("lid", layer.Uint32()),
				zap.Error(err),
			)
		} else {
			h.log.Debug("terminated",
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
	vrf := h.oracle.active(h.signer.NodeID(), layer, IterRound{Round: preround})
	walltime := h.nodeclock.LayerToTime(layer).Add(h.config.PreroundDelay)
	var proposals []types.ProposalID
	if vrf != nil {
		h.log.Debug("active in preround", zap.Uint32("lid", layer.Uint32()))
		// initial set is not needed if node is not active in preround
		select {
		case <-h.wallclock.After(h.wallclock.Until(walltime)):
		case <-h.ctx.Done():
		}
		proposals = h.proposals(layer, beacon)
	}
	proto := newProtocol(proposals, h.config.Committee/2+1)
	if err := h.onOutput(layer, proto.next(vrf != nil), vrf); err != nil {
		return err
	}
	walltime = walltime.Add(h.config.RoundDuration)
	h.log.Debug("ready to accept messages", zap.Uint32("lid", layer.Uint32()))
	for {
		select {
		case input := <-inputs:
			gossip, equivocation := proto.onMessage(input.input)
			h.log.Debug("on message",
				zap.Inline(input.input),
				zap.Bool("gossip", gossip),
			)
			input.result <- &response{gossip: gossip, equivocation: equivocation}
		case <-h.wallclock.After(h.wallclock.Until(walltime)):
			h.log.Debug("execute round",
				zap.Uint32("lid", layer.Uint32()),
				zap.Stringer("round", proto.Round),
			)
			vrf := h.oracle.active(h.signer.NodeID(), layer, proto.IterRound)
			out := proto.next(vrf != nil)
			if err := h.onOutput(layer, out, vrf); err != nil {
				return err
			}
			if out.terminated {
				return nil
			}
			if proto.Iter == h.config.IterationsLimit {
				return fmt.Errorf("hare failed to reach consensus in %d iterations",
					h.config.IterationsLimit)
			}
			walltime = walltime.Add(h.config.RoundDuration)
		case <-h.ctx.Done():
			return nil
		}
	}
}

func (h *Hare) onOutput(layer types.LayerID, out output, vrf *types.HareEligibility) error {
	h.log.Debug("round output",
		zap.Uint32("lid", layer.Uint32()),
		zap.Inline(&out),
		zap.Bool("active", vrf != nil),
	)
	if out.message != nil {
		if vrf == nil {
			panic("inconsistent state. message without vrf")
		}
		out.message.Layer = layer
		out.message.Eligibility = *vrf
		h.eg.Go(func() error {
			out.message.Sender = h.signer.NodeID()
			out.message.Signature = h.signer.Sign(signing.HARE, out.message.ToMetadata().ToBytes())
			if err := h.pubsub.Publish(h.ctx, h.config.ProtocolName, out.message.ToBytes()); err != nil {
				h.log.Error("failed to publish", zap.Inline(out.message))
			}
			return nil
		})
	}
	if out.coin != nil {
		select {
		case <-h.ctx.Done():
			return h.ctx.Err()
		case h.coins <- WeakCoinOutput{Layer: layer, Coin: *out.coin}:
		}
	}
	if out.result != nil {
		select {
		case <-h.ctx.Done():
			return h.ctx.Err()
		case h.results <- ConsensusOutput{Layer: layer, Proposals: out.result}:
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
