// Package miner is responsible for creating valid blocks that contain valid activation transactions and transactions
package miner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/events"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/miner/minweight"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/proposals"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/activesets"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/beacons"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/certificates"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
	"github.com/spacemeshos/go-spacemesh/sql/localsql/activeset"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/tortoise"
)

var errAtxNotAvailable = errors.New("atx not available")

//go:generate mockgen -typed -package=mocks -destination=./mocks/mocks.go -source=./proposal_builder.go

type conservativeState interface {
	SelectProposalTXs(types.LayerID, int) []types.TransactionID
}

type votesEncoder interface {
	LatestComplete() types.LayerID
	TallyVotes(context.Context, types.LayerID)
	EncodeVotes(context.Context, ...tortoise.EncodeVotesOpts) (*types.Opinion, error)
}

type layerClock interface {
	AwaitLayer(layerID types.LayerID) <-chan struct{}
	CurrentLayer() types.LayerID
	LayerToTime(types.LayerID) time.Time
}

// ProposalBuilder builds Proposals for a miner.
type ProposalBuilder struct {
	logger log.Log
	cfg    config

	db        sql.Executor
	localdb   sql.Executor
	atxsdata  *atxsdata.Data
	clock     layerClock
	publisher pubsub.Publisher
	conState  conservativeState
	tortoise  votesEncoder
	syncer    system.SyncStateProvider

	signers struct {
		mu      sync.Mutex
		signers map[types.NodeID]*signerSession
	}
	shared sharedSession

	fallback struct {
		mu   sync.Mutex
		data map[types.EpochID][]types.ATXID
	}

	preparedActivesetLinearizer sync.Mutex
}

type signerSession struct {
	signer  *signing.EdSigner
	log     log.Log
	session session
	latency latencyTracker
}

// shared data for all signers in the epoch.
type sharedSession struct {
	epoch  types.EpochID
	beacon types.Beacon
	active struct {
		id     types.Hash32
		set    types.ATXIDList
		weight uint64
	}
}

// session per every signing key for the whole epoch.
type session struct {
	epoch         types.EpochID
	atx           types.ATXID
	atxWeight     uint64
	ref           types.BallotID
	beacon        types.Beacon
	prev          types.LayerID
	nonce         types.VRFPostIndex
	eligibilities struct {
		proofs map[types.LayerID][]types.VotingEligibility
		slots  uint32
	}
}

func (s *session) MarshalLogObject(encoder log.ObjectEncoder) error {
	encoder.AddUint32("epoch", s.epoch.Uint32())
	encoder.AddString("beacon", s.beacon.String())
	encoder.AddString("atx", s.atx.ShortString())
	encoder.AddUint64("weight", s.atxWeight)
	if s.ref != types.EmptyBallotID {
		encoder.AddString("ref", s.ref.String())
	}
	encoder.AddUint32("prev", s.prev.Uint32())
	if s.eligibilities.proofs != nil {
		encoder.AddUint32("slots", s.eligibilities.slots)
		encoder.AddInt("eligible", len(s.eligibilities.proofs))
		encoder.AddArray(
			"eligible by layer",
			log.ArrayMarshalerFunc(func(encoder log.ArrayEncoder) error {
				// Sort the layer map to log the layer data in order
				keys := maps.Keys(s.eligibilities.proofs)
				slices.Sort(keys)
				for _, lyr := range keys {
					encoder.AppendObject(
						log.ObjectMarshallerFunc(func(encoder log.ObjectEncoder) error {
							encoder.AddUint32("layer", lyr.Uint32())
							encoder.AddInt("slots", len(s.eligibilities.proofs[lyr]))
							return nil
						}),
					)
				}
				return nil
			}),
		)
	}
	return nil
}

// config defines configuration for the ProposalBuilder.
type config struct {
	layerSize          uint32
	layersPerEpoch     uint32
	hdist              uint32
	networkDelay       time.Duration
	workersLimit       int
	minActiveSetWeight []types.EpochMinimalActiveWeight
	// used to determine whether a node has enough information on the active set this epoch
	goodAtxPercent int
	activeSet      ActiveSetPreparation
}

// ActiveSetPreparation is a configuration to enable computation of activeset in advance.
type ActiveSetPreparation struct {
	// Window describes how much in advance the active set should be prepared.
	Window time.Duration `mapstructure:"window"`
	// RetryInterval describes how often the active set is retried.
	RetryInterval time.Duration `mapstructure:"retry-interval"`
	// Tries describes how many times the active set is retried.
	Tries int `mapstructure:"tries"`
}

func DefaultActiveSetPrepartion() ActiveSetPreparation {
	return ActiveSetPreparation{
		Window:        1 * time.Second,
		RetryInterval: 1 * time.Second,
		Tries:         3,
	}
}

func (c *config) MarshalLogObject(encoder log.ObjectEncoder) error {
	encoder.AddUint32("layer size", c.layerSize)
	encoder.AddUint32("epoch size", c.layersPerEpoch)
	encoder.AddUint32("hdist", c.hdist)
	encoder.AddDuration("network delay", c.networkDelay)
	encoder.AddInt("good atx percent", c.goodAtxPercent)
	encoder.AddDuration("active set window", c.activeSet.Window)
	encoder.AddDuration("active set retry interval", c.activeSet.RetryInterval)
	encoder.AddInt("active set tries", c.activeSet.Tries)
	return nil
}

// Opt for configuring ProposalBuilder.
type Opt func(h *ProposalBuilder)

// WithLayerSize defines the average number of proposal per layer.
func WithLayerSize(size uint32) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.layerSize = size
	}
}

// WithWorkersLimit configures paralelization factor for builder operation when working with
// more than one signer.
func WithWorkersLimit(limit int) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.workersLimit = limit
	}
}

// WithLayerPerEpoch defines the number of layers per epoch.
func WithLayerPerEpoch(layers uint32) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.layersPerEpoch = layers
	}
}

func WithMinimalActiveSetWeight(weight []types.EpochMinimalActiveWeight) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.minActiveSetWeight = weight
	}
}

// WithLogger defines the logger.
func WithLogger(logger log.Log) Opt {
	return func(pb *ProposalBuilder) {
		pb.logger = logger
	}
}

func WithHdist(dist uint32) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.hdist = dist
	}
}

func WithNetworkDelay(delay time.Duration) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.networkDelay = delay
	}
}

func WithMinGoodAtxPercent(percent int) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.goodAtxPercent = percent
	}
}

// WithSigners guarantees that builder will start execution with provided list of signers.
// Should be after logging.
func WithSigners(signers ...*signing.EdSigner) Opt {
	return func(pb *ProposalBuilder) {
		for _, signer := range signers {
			pb.Register(signer)
		}
	}
}

// WithActiveSetPrepation overwrites configuration for activeset preparation.
func WithActiveSetPrepation(prep ActiveSetPreparation) Opt {
	return func(pb *ProposalBuilder) {
		pb.cfg.activeSet = prep
	}
}

// New creates a struct of block builder type.
func New(
	clock layerClock,
	db sql.Executor,
	localdb sql.Executor,
	atxsdata *atxsdata.Data,
	publisher pubsub.Publisher,
	trtl votesEncoder,
	syncer system.SyncStateProvider,
	conState conservativeState,
	opts ...Opt,
) *ProposalBuilder {
	pb := &ProposalBuilder{
		cfg: config{
			workersLimit: runtime.NumCPU(),
			activeSet:    DefaultActiveSetPrepartion(),
		},
		logger:    log.NewNop(),
		clock:     clock,
		db:        db,
		localdb:   localdb,
		atxsdata:  atxsdata,
		publisher: publisher,
		tortoise:  trtl,
		syncer:    syncer,
		conState:  conState,
		signers: struct {
			mu      sync.Mutex
			signers map[types.NodeID]*signerSession
		}{
			signers: map[types.NodeID]*signerSession{},
		},
		fallback: struct {
			mu   sync.Mutex
			data map[types.EpochID][]types.ATXID
		}{
			data: map[types.EpochID][]types.ATXID{},
		},
	}
	for _, opt := range opts {
		opt(pb)
	}
	return pb
}

func (pb *ProposalBuilder) Register(signer *signing.EdSigner) {
	pb.signers.mu.Lock()
	defer pb.signers.mu.Unlock()
	_, exist := pb.signers.signers[signer.NodeID()]
	if !exist {
		pb.signers.signers[signer.NodeID()] = &signerSession{
			signer: signer,
			log:    pb.logger.WithFields(log.String("signer", signer.NodeID().ShortString())),
		}
	}
}

// Start the loop that listens to layers and build proposals.
func (pb *ProposalBuilder) Run(ctx context.Context) error {
	current := pb.clock.CurrentLayer()
	next := current + 1
	pb.logger.With().Info("started", log.Inline(&pb.cfg), log.Uint32("next", next.Uint32()))
	var eg errgroup.Group
	if pb.cfg.activeSet.Tries == 0 || pb.cfg.activeSet.RetryInterval == 0 {
		pb.logger.With().Warning("activeset will not be prepared in advance")
	} else {
		eg.Go(func() error {
			// check that activeset was prepared for current epoch
			pb.ensureActiveSetPrepared(ctx, current.GetEpoch())
			// and then wait for the right time to generate activeset for the next epoch
			wait := time.Until(
				pb.clock.LayerToTime((current.GetEpoch() + 1).FirstLayer()).Add(-pb.cfg.activeSet.Window))
			if wait > 0 {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(wait):
				}
			}
			pb.ensureActiveSetPrepared(ctx, current.GetEpoch())
			return nil
		})
	}
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-pb.clock.AwaitLayer(next):
				current := pb.clock.CurrentLayer()
				if current.Before(next) {
					pb.logger.With().Info("time sync detected, realigning ProposalBuilder",
						log.Uint32("current", current.Uint32()),
						log.Uint32("next", next.Uint32()),
					)
					continue
				}
				next = current.Add(1)
				ctx := log.WithNewSessionID(ctx)
				if current <= types.GetEffectiveGenesis() || !pb.syncer.IsSynced(ctx) {
					continue
				}
				if err := pb.build(ctx, current); err != nil {
					if errors.Is(err, errAtxNotAvailable) {
						pb.logger.With().
							Debug("signer is not active in epoch", log.Context(ctx), log.Uint32("lid", current.Uint32()), log.Err(err))
					} else {
						pb.logger.With().Warning("failed to build proposal",
							log.Context(ctx), log.Uint32("lid", current.Uint32()), log.Err(err),
						)
					}
				}
			}
		}
	})
	return eg.Wait()
}

// only output the mesh hash in the proposal when the following conditions are met:
// - tortoise has verified every layer i < N-hdist.
// - the node has hare output for every layer i such that N-hdist <= i <= N.
// this is done such that when the node is generating the block based on hare output,
// it can do optimistic filtering if the majority of the proposals agreed on the mesh hash.
func (pb *ProposalBuilder) decideMeshHash(ctx context.Context, current types.LayerID) types.Hash32 {
	var minVerified types.LayerID
	if current.Uint32() > pb.cfg.hdist+1 {
		minVerified = current.Sub(pb.cfg.hdist + 1)
	}
	genesis := types.GetEffectiveGenesis()
	if minVerified.Before(genesis) {
		minVerified = genesis
	}
	verified := pb.tortoise.LatestComplete()
	if minVerified.After(verified) {
		pb.logger.With().Warning("layers outside hdist not verified",
			log.Context(ctx),
			current,
			log.Stringer("min verified", minVerified),
			log.Stringer("latest verified", verified))
		return types.EmptyLayerHash
	}
	pb.logger.With().Debug("verified layer meets optimistic filtering threshold",
		log.Context(ctx),
		current,
		log.Stringer("min verified", minVerified),
		log.Stringer("latest verified", verified),
	)

	for lid := minVerified.Add(1); lid.Before(current); lid = lid.Add(1) {
		_, err := certificates.GetHareOutput(pb.db, lid)
		if err != nil {
			pb.logger.With().Warning("missing hare output for layer within hdist",
				log.Context(ctx),
				current,
				log.Stringer("missing_layer", lid),
				log.Err(err),
			)
			return types.EmptyLayerHash
		}
	}
	pb.logger.With().Debug("hare outputs meet optimistic filtering threshold",
		log.Context(ctx),
		current,
		log.Stringer("from", minVerified.Add(1)),
		log.Stringer("to", current.Sub(1)),
	)

	mesh, err := layers.GetAggregatedHash(pb.db, current.Sub(1))
	if err != nil {
		pb.logger.With().Warning("failed to get mesh hash",
			log.Context(ctx),
			current,
			log.Err(err),
		)
		return types.EmptyLayerHash
	}
	return mesh
}

func (pb *ProposalBuilder) UpdateActiveSet(epoch types.EpochID, activeSet []types.ATXID) {
	pb.logger.With().Info("received trusted activeset update",
		epoch,
		log.Int("size", len(activeSet)),
	)
	pb.fallback.mu.Lock()
	defer pb.fallback.mu.Unlock()
	if _, ok := pb.fallback.data[epoch]; ok {
		pb.logger.With().Debug("fallback active set already exists", epoch)
		return
	}
	pb.fallback.data[epoch] = activeSet
}

func (pb *ProposalBuilder) initSharedData(ctx context.Context, current types.LayerID) error {
	if pb.shared.epoch != current.GetEpoch() {
		pb.shared = sharedSession{epoch: current.GetEpoch()}
	}
	if pb.shared.beacon == types.EmptyBeacon {
		beacon, err := beacons.Get(pb.db, pb.shared.epoch)
		if err != nil || beacon == types.EmptyBeacon {
			return fmt.Errorf("missing beacon for epoch %d", pb.shared.epoch)
		}
		pb.shared.beacon = beacon
	}
	if pb.shared.active.set != nil {
		return nil
	}
	id, weight, set, err := pb.prepareActiveSet(current, current.GetEpoch())
	if err != nil {
		return err
	}
	pb.logger.With().Info("loaded prepared active set",
		pb.shared.epoch,
		log.ShortStringer("id", id),
		log.Int("size", len(set)),
		log.Uint64("weight", weight),
	)
	pb.shared.active.id = id
	pb.shared.active.set = set
	pb.shared.active.weight = weight
	return nil
}

func (pb *ProposalBuilder) initSignerData(
	ctx context.Context,
	ss *signerSession,
	lid types.LayerID,
) error {
	if ss.session.epoch != lid.GetEpoch() {
		ss.session = session{epoch: lid.GetEpoch()}
	}
	if ss.session.atx == types.EmptyATXID {
		atxid, err := atxs.GetIDByEpochAndNodeID(pb.db, ss.session.epoch-1, ss.signer.NodeID())
		if err != nil {
			if errors.Is(err, sql.ErrNotFound) {
				err = errAtxNotAvailable
			}
			return fmt.Errorf("get atx in epoch %v: %w", ss.session.epoch-1, err)
		}
		atx := pb.atxsdata.Get(ss.session.epoch, atxid)
		if atx == nil {
			return fmt.Errorf("missing atx in atxsdata %v", atxid)
		}
		ss.session.atx = atxid
		ss.session.atxWeight = atx.Weight
		ss.session.nonce = atx.Nonce
	}
	if ss.session.prev == 0 {
		prev, err := ballots.LastInEpoch(pb.db, ss.session.atx, ss.session.epoch)
		if err != nil && !errors.Is(err, sql.ErrNotFound) {
			return err
		}
		if err == nil {
			ss.session.prev = prev.Layer
		}
	}
	if ss.session.ref == types.EmptyBallotID {
		ballot, err := ballots.FirstInEpoch(pb.db, ss.session.atx, ss.session.epoch)
		if err != nil && !errors.Is(err, sql.ErrNotFound) {
			return fmt.Errorf("get refballot %w", err)
		}
		if errors.Is(err, sql.ErrNotFound) {
			ss.session.beacon = pb.shared.beacon
			ss.session.eligibilities.slots = proposals.MustGetNumEligibleSlots(
				ss.session.atxWeight,
				minweight.Select(lid.GetEpoch(), pb.cfg.minActiveSetWeight),
				pb.shared.active.weight,
				pb.cfg.layerSize,
				pb.cfg.layersPerEpoch,
			)
		} else {
			if ballot.EpochData == nil {
				return fmt.Errorf("atx %d created invalid first ballot", ss.session.atx)
			}
			ss.session.ref = ballot.ID()
			ss.session.beacon = ballot.EpochData.Beacon
			ss.session.eligibilities.slots = ballot.EpochData.EligibilityCount
		}
	}
	if ss.session.eligibilities.proofs == nil {
		ss.session.eligibilities.proofs = calcEligibilityProofs(
			ss.signer.VRFSigner(),
			ss.session.epoch,
			ss.session.beacon,
			ss.session.nonce,
			ss.session.eligibilities.slots,
			pb.cfg.layersPerEpoch,
		)
		ss.log.With().Info("proposal eligibilities for an epoch", log.Inline(&ss.session))
		events.EmitEligibilities(
			ss.session.epoch,
			ss.session.beacon,
			ss.session.atx,
			uint32(len(pb.shared.active.set)),
			ss.session.eligibilities.proofs,
		)
	}
	return nil
}

func (pb *ProposalBuilder) build(ctx context.Context, lid types.LayerID) error {
	start := time.Now()
	if err := pb.initSharedData(ctx, lid); err != nil {
		return err
	}

	pb.signers.mu.Lock()
	// don't accept registration in the middle of computing proposals
	signers := maps.Values(pb.signers.signers)
	pb.signers.mu.Unlock()

	var eg errgroup.Group
	eg.SetLimit(pb.cfg.workersLimit)
	for _, ss := range signers {
		ss := ss
		ss.latency.start = start
		eg.Go(func() error {
			if err := pb.initSignerData(ctx, ss, lid); err != nil {
				if errors.Is(err, errAtxNotAvailable) {
					ss.log.With().Debug("smesher doesn't have atx that targets this epoch",
						log.Context(ctx), ss.session.epoch.Field(),
					)
				} else {
					return err
				}
			}
			if lid <= ss.session.prev {
				return fmt.Errorf(
					"layer %d was already built by signer %s",
					lid,
					ss.signer.NodeID().ShortString(),
				)
			}
			ss.session.prev = lid
			ss.latency.data = time.Now()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	any := false
	for _, ss := range signers {
		if n := len(ss.session.eligibilities.proofs[lid]); n == 0 {
			ss.log.With().Debug("not eligible for proposal in layer",
				log.Context(ctx),
				lid.Field(), lid.GetEpoch().Field())
			continue
		} else {
			ss.log.With().Debug("eligible for proposals in layer",
				log.Context(ctx),
				lid.Field(), log.Int("num proposals", n),
			)
			any = true
		}
	}
	if !any {
		return nil
	}

	pb.tortoise.TallyVotes(ctx, lid)
	// TODO(dshulyak) get rid from the EncodeVotesWithCurrent option in a followup
	// there are some dependencies in the tests
	opinion, err := pb.tortoise.EncodeVotes(ctx, tortoise.EncodeVotesWithCurrent(lid))
	if err != nil {
		return fmt.Errorf("encode votes: %w", err)
	}
	for _, ss := range signers {
		ss.latency.tortoise = time.Now()
	}

	meshHash := pb.decideMeshHash(ctx, lid)
	for _, ss := range signers {
		ss.latency.hash = time.Now()
	}

	for _, ss := range signers {
		proofs := ss.session.eligibilities.proofs[lid]
		if len(proofs) == 0 {
			ss.log.With().Debug("not eligible for proposal in layer",
				log.Context(ctx),
				lid.Field(), lid.GetEpoch().Field())
			continue
		}
		ss.log.With().Debug("eligible for proposals in layer",
			log.Context(ctx),
			lid.Field(), log.Int("num proposals", len(proofs)),
		)

		txs := pb.conState.SelectProposalTXs(lid, len(proofs))
		ss.latency.txs = time.Now()

		// needs to be saved before publishing, as we will query it in handler
		if ss.session.ref == types.EmptyBallotID {
			if err := activesets.Add(pb.db, pb.shared.active.id, &types.EpochActiveSet{
				Epoch: ss.session.epoch,
				Set:   pb.shared.active.set,
			}); err != nil && !errors.Is(err, sql.ErrObjectExists) {
				return err
			}
		}

		ss := ss
		eg.Go(func() error {
			proposal := createProposal(
				&ss.session,
				pb.shared.beacon,
				pb.shared.active.set,
				ss.signer,
				lid,
				txs,
				opinion,
				proofs,
				meshHash,
			)
			if err := pb.publisher.Publish(ctx, pubsub.ProposalProtocol, codec.MustEncode(proposal)); err != nil {
				ss.log.Error("failed to publish proposal",
					log.Context(ctx),
					log.Uint32("lid", proposal.Layer.Uint32()),
					log.Stringer("id", proposal.ID()),
					log.Err(err),
				)
			} else {
				ss.latency.publish = time.Now()
				ss.log.With().Info("proposal created", log.Context(ctx), log.Inline(proposal), log.Object("latency", &ss.latency))
				proposalBuild.Observe(ss.latency.total().Seconds())
				events.EmitProposal(lid, proposal.ID())
				events.ReportProposal(events.ProposalCreated, proposal)
			}
			return nil
		})
	}
	return eg.Wait()
}

func createProposal(
	session *session,
	beacon types.Beacon,
	activeset types.ATXIDList,
	signer *signing.EdSigner,
	lid types.LayerID,
	txs []types.TransactionID,
	opinion *types.Opinion,
	eligibility []types.VotingEligibility,
	meshHash types.Hash32,
) *types.Proposal {
	p := &types.Proposal{
		InnerProposal: types.InnerProposal{
			Ballot: types.Ballot{
				InnerBallot: types.InnerBallot{
					Layer:       lid,
					AtxID:       session.atx,
					OpinionHash: opinion.Hash,
				},
				Votes:             opinion.Votes,
				EligibilityProofs: eligibility,
			},
			TxIDs:    txs,
			MeshHash: meshHash,
		},
	}
	if session.ref == types.EmptyBallotID {
		p.Ballot.RefBallot = types.EmptyBallotID
		p.Ballot.EpochData = &types.EpochData{
			ActiveSetHash:    activeset.Hash(),
			Beacon:           beacon,
			EligibilityCount: session.eligibilities.slots,
		}
	} else {
		p.Ballot.RefBallot = session.ref
	}
	p.Ballot.Signature = signer.Sign(signing.BALLOT, p.Ballot.SignedBytes())
	p.SmesherID = signer.NodeID()
	p.Signature = signer.Sign(signing.PROPOSAL, p.SignedBytes())
	p.MustInitialize()
	return p
}

func ActiveSetFromEpochFirstBlock(db sql.Executor, epoch types.EpochID) ([]types.ATXID, error) {
	bid, err := layers.FirstAppliedInEpoch(db, epoch)
	if err != nil {
		return nil, fmt.Errorf("first block in epoch %d not found: %w", epoch, err)
	}
	return activeSetFromBlock(db, bid)
}

func activeSetFromBlock(db sql.Executor, bid types.BlockID) ([]types.ATXID, error) {
	block, err := blocks.Get(db, bid)
	if err != nil {
		return nil, fmt.Errorf("actives get block: %w", err)
	}
	activeMap := make(map[types.ATXID]struct{})
	// the active set is the union of all active sets recorded in rewarded miners' ref ballot
	for _, r := range block.Rewards {
		activeMap[r.AtxID] = struct{}{}
		ballot, err := ballots.FirstInEpoch(db, r.AtxID, block.LayerIndex.GetEpoch())
		if err != nil {
			return nil, fmt.Errorf("actives get ballot: %w", err)
		}
		actives, err := activesets.Get(db, ballot.EpochData.ActiveSetHash)
		if err != nil {
			return nil, fmt.Errorf(
				"actives get active hash for ballot %s: %w",
				ballot.ID().String(),
				err,
			)
		}
		for _, id := range actives.Set {
			activeMap[id] = struct{}{}
		}
	}
	return maps.Keys(activeMap), nil
}

func (pb *ProposalBuilder) ensureActiveSetPrepared(ctx context.Context, target types.EpochID) {
	var err error
	for try := 0; try < pb.cfg.activeSet.Tries; try++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pb.cfg.activeSet.RetryInterval):
		}
		current := pb.clock.CurrentLayer()
		// we run it here for side effects
		_, _, _, err = pb.prepareActiveSet(current, target)
		if err == nil {
			return
		}
		pb.logger.With().Debug("failed to prepare active set", log.Err(err), log.Uint32("attempt", uint32(try)))
	}
	pb.logger.With().Warning("failed to prepare active set", log.Err(err))
}

// prepareActiveSet generates activeset in advance.
//
// It stores it on the builder and persists it in the database, so that when node is restarted
// it doesn't have to redo the work.
//
// The method is expected to be called at any point in target epoch, as well as the very end of the previous epoch.
func (pb *ProposalBuilder) prepareActiveSet(
	current types.LayerID,
	target types.EpochID,
) (types.Hash32, uint64, []types.ATXID, error) {
	pb.preparedActivesetLinearizer.Lock()
	defer pb.preparedActivesetLinearizer.Unlock()

	id, setWeight, set, err := activeset.Get(pb.localdb, activeset.Tortoise, target)
	if err != nil && !errors.Is(err, sql.ErrNotFound) {
		return id, 0, nil, fmt.Errorf("failed to get prepared active set: %w", err)
	}
	if err == nil {
		return id, setWeight, set, nil
	}

	pb.fallback.mu.Lock()
	fallback, exists := pb.fallback.data[target]
	pb.fallback.mu.Unlock()

	start := time.Now()
	if exists {
		pb.logger.With().Info("generating activeset from trusted fallback",
			target.Field(),
			log.Int("size", len(fallback)),
		)
		var err error
		setWeight, err = getSetWeight(pb.atxsdata, target, fallback)
		if err != nil {
			return id, 0, nil, err
		}
		set = fallback
	} else {
		epochStart := pb.clock.LayerToTime(target.FirstLayer())
		networkDelay := pb.cfg.networkDelay
		pb.logger.With().Info("generating activeset from grades", target.Field(),
			log.Time("epoch start", epochStart),
			log.Duration("network delay", networkDelay),
		)
		result, err := activeSetFromGrades(pb.db, target, epochStart, networkDelay)
		if err != nil {
			return id, 0, nil, err
		}
		if result.Total == 0 {
			return id, 0, nil, fmt.Errorf("empty active set")
		}
		if result.Total > 0 && len(result.Set)*100/result.Total > pb.cfg.goodAtxPercent {
			set = result.Set
			setWeight = result.Weight
		} else {
			pb.logger.With().Info("node was not synced during previous epoch. can't use activeset from grades",
				target.Field(),
				log.Time("epoch start", epochStart),
				log.Duration("network delay", networkDelay),
				log.Int("total", result.Total),
				log.Int("set", len(result.Set)),
				log.Int("omitted", result.Total-len(result.Set)),
			)
		}
	}
	if set == nil && current > target.FirstLayer() {
		pb.logger.With().Info("generating activeset from first block",
			log.Uint32("current", current.Uint32()),
			log.Uint32("first", target.FirstLayer().Uint32()),
		)
		// see how it can be further improved https://github.com/spacemeshos/go-spacemesh/issues/5560
		var err error
		set, err = ActiveSetFromEpochFirstBlock(pb.db, pb.shared.epoch)
		if err != nil {
			return id, 0, nil, err
		}
		setWeight, err = getSetWeight(pb.atxsdata, pb.shared.epoch, set)
		if err != nil {
			return id, 0, nil, err
		}
	}
	if set != nil && setWeight == 0 {
		return id, 0, nil, fmt.Errorf("empty active set")
	}
	if set != nil {
		pb.logger.With().Info("prepared activeset",
			target.Field(),
			log.Int("size", len(set)),
			log.Uint64("weight", setWeight),
			log.Duration("elapsed", time.Since(start)),
		)
		sort.Slice(set, func(i, j int) bool {
			return bytes.Compare(set[i].Bytes(), set[j].Bytes()) < 0
		})
		id := types.ATXIDList(set).Hash()
		if err := activeset.Add(pb.localdb, activeset.Tortoise, target, id, setWeight, set); err != nil {
			return id, 0, nil, fmt.Errorf("failed to persist prepared active set for epoch %v: %w", target, err)
		}
		return id, setWeight, set, nil
	}
	return id, 0, nil, fmt.Errorf("failed to generate activeset")
}

type gradedActiveSet struct {
	// Set includes activations with highest grade.
	Set []types.ATXID
	// Weight of the activations in the Set.
	Weight uint64
	// Total number of activations in the database that targets requests epoch.
	Total int
}

// activeSetFromGrades includes activations with the highest grade.
// Such activations were received atleast 4 network delays before the epoch start, and no malfeasence proof for
// identity was received before the epoch start.
//
// On mainnet we use 30minutes as a network delay parameter.
func activeSetFromGrades(
	db sql.Executor,
	target types.EpochID,
	epochStart time.Time,
	networkDelay time.Duration,
) (gradedActiveSet, error) {
	var (
		setWeight uint64
		set       []types.ATXID
		total     int
	)
	if err := atxs.IterateForGrading(db, target-1, func(id types.ATXID, atxtime, prooftime int64, weight uint64) bool {
		total++
		if gradeAtx(epochStart, networkDelay, atxtime, prooftime) == good {
			set = append(set, id)
			setWeight += weight
		}
		return true
	}); err != nil {
		return gradedActiveSet{}, fmt.Errorf("failed to iterate atxs that target epoch %v: %v", target, err)
	}
	return gradedActiveSet{
		Set:    set,
		Weight: setWeight,
		Total:  total,
	}, nil
}

func getSetWeight(atxsdata *atxsdata.Data, target types.EpochID, set []types.ATXID) (uint64, error) {
	var setWeight uint64
	for _, id := range set {
		atx := atxsdata.Get(target, id)
		if atx == nil {
			return 0, fmt.Errorf("atx %s/%s is missing in atxsdata", target, id.ShortString())
		}
		setWeight += atx.Weight
	}
	return setWeight, nil
}

// calcEligibilityProofs calculates the eligibility proofs of proposals for the miner in the given epoch
// and returns the proofs along with the epoch's active set.
func calcEligibilityProofs(
	signer *signing.VRFSigner,
	epoch types.EpochID,
	beacon types.Beacon,
	nonce types.VRFPostIndex,
	slots uint32,
	layersPerEpoch uint32,
) map[types.LayerID][]types.VotingEligibility {
	proofs := map[types.LayerID][]types.VotingEligibility{}
	for counter := uint32(0); counter < slots; counter++ {
		vrf := signer.Sign(proposals.MustSerializeVRFMessage(beacon, epoch, nonce, counter))
		layer := proposals.CalcEligibleLayer(epoch, layersPerEpoch, vrf)
		proofs[layer] = append(proofs[layer], types.VotingEligibility{
			J:   counter,
			Sig: vrf,
		})
	}
	return proofs
}

// atxGrade describes the grade of an ATX as described in
// https://community.spacemesh.io/t/grading-atxs-for-the-active-set/335
//
// let s be the start of the epoch, and δ the network propagation time.
// grade 0: ATX was received at time t >= s-3δ, or an equivocation proof was received by time s-δ.
// grade 1: ATX was received at time t < s-3δ before the start of the epoch, and no equivocation proof by time s-δ.
// grade 2: ATX was received at time t < s-4δ, and no equivocation proof was received for that id until time s.
type atxGrade int

const (
	evil atxGrade = iota
	acceptable
	good
)

func gradeAtx(epochStart time.Time, networkDelay time.Duration, atxtime, prooftime int64) atxGrade {
	atx := time.Unix(0, atxtime)
	proof := time.Unix(0, prooftime)
	if atx.Before(epochStart.Add(-4*networkDelay)) && (prooftime == 0 || !proof.Before(epochStart)) {
		return good
	} else if atx.Before(epochStart.Add(-3*networkDelay)) &&
		(proof.IsZero() || !proof.Before(epochStart.Add(-networkDelay))) {
		return acceptable
	}
	return evil
}
