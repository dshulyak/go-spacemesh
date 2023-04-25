package tortoise

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/spacemeshos/fixed"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/log"
	putil "github.com/spacemeshos/go-spacemesh/proposals/util"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/certificates"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
	"github.com/spacemeshos/go-spacemesh/system"
	"github.com/spacemeshos/go-spacemesh/tortoise/metrics"
)

var errBeaconUnavailable = errors.New("beacon unavailable")

type turtle struct {
	Config
	logger log.Log
	cdb    *datastore.CachedDB

	beacons system.BeaconGetter
	updated map[types.LayerID]map[types.BlockID]bool

	*state

	// a linked list with retriable ballots
	// the purpose is to add ballot to the state even
	// if beacon is not available locally, as tortoise
	// can't count ballots without knowing local beacon
	retriable list.List

	verifying *verifying

	isFull bool
	full   *full
}

// newTurtle creates a new verifying tortoise algorithm instance.
func newTurtle(
	logger log.Log,
	cdb *datastore.CachedDB,
	beacons system.BeaconGetter,
	config Config,
) *turtle {
	t := &turtle{
		Config:  config,
		state:   newState(),
		logger:  logger,
		cdb:     cdb,
		beacons: beacons,
	}
	genesis := types.GetEffectiveGenesis()

	t.last = genesis
	t.processed = genesis
	t.verified = genesis
	t.evicted = genesis.Sub(1)

	t.epochs[genesis.GetEpoch()] = &epochInfo{atxs: map[types.ATXID]uint64{}}
	t.layers[genesis] = &layerInfo{
		lid:            genesis,
		hareTerminated: true,
	}
	t.verifying = newVerifying(config, t.state)
	t.full = newFullTortoise(config, t.state)
	t.full.counted = genesis

	gen := t.layer(genesis)
	gen.computeOpinion(t.Hdist, t.last)
	return t
}

func (t *turtle) lookbackWindowStart() (types.LayerID, bool) {
	// prevent overflow/wraparound
	if t.verified.Before(types.LayerID(t.WindowSize)) {
		return types.LayerID(0), false
	}
	return t.verified.Sub(t.WindowSize), true
}

// evict makes sure we only keep a window of the last hdist layers.
func (t *turtle) evict(ctx context.Context) {
	// Don't evict before we've verified at least hdist layers
	if !t.verified.After(types.GetEffectiveGenesis().Add(t.Hdist)) {
		return
	}
	// TODO: fix potential leak when we can't verify but keep receiving layers
	//    see https://github.com/spacemeshos/go-spacemesh/issues/2671

	windowStart, ok := t.lookbackWindowStart()
	if !ok {
		return
	}
	if !windowStart.After(t.evicted) {
		return
	}

	t.logger.With().Debug("evict in memory state",
		log.Stringer("from_layer", t.evicted.Add(1)),
		log.Stringer("upto_layer", windowStart),
	)

	for lid := t.evicted.Add(1); lid.Before(windowStart); lid = lid.Add(1) {
		for _, ballot := range t.ballots[lid] {
			ballotsNumber.Dec()
			delete(t.ballotRefs, ballot.id)
		}
		for range t.layers[lid].blocks {
			blocksNumber.Dec()
		}
		layersNumber.Dec()
		delete(t.layers, lid)
		delete(t.ballots, lid)
		if lid.OrdinalInEpoch() == types.GetLayersPerEpoch()-1 {
			layersNumber.Dec()
			delete(t.epochs, lid.GetEpoch())
		}
	}
	for _, ballot := range t.ballots[windowStart] {
		ballot.votes.cutBefore(windowStart)
	}
	t.evicted = windowStart.Sub(1)
	evictedLayer.Set(float64(t.evicted))
}

// EncodeVotes by choosing base ballot and explicit votes.
func (t *turtle) EncodeVotes(ctx context.Context, conf *encodeConf) (*types.Opinion, error) {
	if err := t.checkDrained(); err != nil {
		return nil, err
	}
	var (
		logger = t.logger.WithContext(ctx)
		err    error

		current = t.last.Add(1)
	)
	if conf.current != nil {
		current = *conf.current
	}
	for lid := current.Sub(1); lid.After(t.evicted); lid = lid.Sub(1) {
		var choices []*ballotInfo
		if lid == types.GetEffectiveGenesis() {
			choices = []*ballotInfo{{layer: types.GetEffectiveGenesis()}}
		} else {
			choices = t.ballots[lid]
		}
		for _, base := range choices {
			if base.malicious {
				// skip them as they are candidates for pruning
				continue
			}
			var opinion *types.Opinion
			opinion, err = t.encodeVotes(ctx, base, t.evicted.Add(1), current)
			if err == nil {
				metrics.LayerDistanceToBaseBallot.WithLabelValues().Observe(float64(t.last - base.layer))
				logger.With().Info("encoded votes",
					log.Stringer("base ballot", base.id),
					log.Stringer("base layer", base.layer),
					log.Stringer("voting layer", current),
					log.Inline(opinion),
				)
				return opinion, nil
			}
			logger.With().Debug("failed to encode votes using base ballot id",
				base.id,
				log.Err(err),
				log.Stringer("current layer", current),
			)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode votes: %w", err)
	}
	return nil, fmt.Errorf("no ballots within a sliding window")
}

// encode differences between selected base ballot and local votes.
func (t *turtle) encodeVotes(
	ctx context.Context,
	base *ballotInfo,
	start types.LayerID,
	current types.LayerID,
) (*types.Opinion, error) {
	logger := t.logger.WithContext(ctx).WithFields(
		log.Stringer("base layer", base.layer),
		log.Stringer("current layer", current),
	)
	votes := types.Votes{
		Base: base.id,
	}
	// encode difference with local opinion between [start, base.layer)
	for lvote := base.votes.tail; lvote != nil; lvote = lvote.prev {
		if lvote.lid.Before(start) {
			break
		}
		layer := t.layer(lvote.lid)
		if lvote.vote == abstain && layer.hareTerminated {
			return nil, fmt.Errorf("ballot %s can't be used as a base ballot", base.id)
		}
		if lvote.vote != abstain && !layer.hareTerminated {
			logger.With().Debug("voting abstain on the layer", lvote.lid)
			votes.Abstain = append(votes.Abstain, lvote.lid)
			continue
		}
		for _, block := range layer.blocks {
			vote, reason, err := t.getFullVote(t.verified, current, block)
			if err != nil {
				return nil, err
			}
			// ballot vote is consistent with local opinion, exception is not necessary
			bvote := lvote.getVote(block)
			if vote == bvote {
				continue
			}
			switch vote {
			case support:
				logger.With().Debug("support before base ballot", log.Inline(block))
				votes.Support = append(votes.Support, block.header())
			case against:
				logger.With().Debug("explicit against overwrites base ballot opinion", log.Inline(block))
				votes.Against = append(votes.Against, block.header())
			case abstain:
				logger.With().Error("layers that are not terminated should have been encoded earlier",
					log.Inline(block), log.Stringer("reason", reason),
				)
			}
		}
	}
	// encode votes after base ballot votes [base layer, last)
	for lid := base.layer; lid.Before(current); lid = lid.Add(1) {
		layer := t.layer(lid)
		if !layer.hareTerminated {
			logger.With().Debug("voting abstain on the layer", lid)
			votes.Abstain = append(votes.Abstain, lid)
			continue
		}
		for _, block := range layer.blocks {
			vote, reason, err := t.getFullVote(t.verified, current, block)
			if err != nil {
				return nil, err
			}
			switch vote {
			case support:
				logger.With().Debug("support after base ballot", log.Inline(block), log.Stringer("reason", reason))
				votes.Support = append(votes.Support, block.header())
			case against:
				logger.With().Debug("implicit against after base ballot", log.Inline(block), log.Stringer("reason", reason))
			case abstain:
				logger.With().Error("layers that are not terminated should have been encoded earlier",
					log.Inline(block), log.Stringer("reason", reason),
				)
			}
		}
	}

	if explen := len(votes.Support) + len(votes.Against); explen > t.MaxExceptions {
		return nil, fmt.Errorf("too many exceptions (%v)", explen)
	}
	decoded, _, err := decodeVotes(t.evicted, current, base, votes)
	if err != nil {
		return nil, err
	}
	return &types.Opinion{
		Hash:  decoded.opinion(),
		Votes: votes,
	}, nil
}

// getFullVote unlike getLocalVote will vote according to the counted votes on blocks that are
// outside of hdist. if opinion is undecided according to the votes it will use coinflip recorded
// in the current layer.
func (t *turtle) getFullVote(verified, current types.LayerID, block *blockInfo) (sign, voteReason, error) {
	if !block.data {
		return against, reasonMissingData, nil
	}
	vote, reason := getLocalVote(t.Config, verified, current, block)
	if !(vote == abstain && reason == reasonValidity) {
		return vote, reason, nil
	}
	vote = crossesThreshold(block.margin, t.localThreshold)
	if vote != abstain {
		return vote, reasonLocalThreshold, nil
	}
	coin, err := layers.GetWeakCoin(t.cdb, current.Sub(1))
	if err != nil {
		return 0, "", fmt.Errorf("coinflip is not recorded in %s. required for vote on %s / %s",
			current.Sub(1), block.id, block.layer)
	}
	if coin {
		return support, reasonCoinflip, nil
	}
	return against, reasonCoinflip, nil
}

func (t *turtle) onLayer(ctx context.Context, last types.LayerID) error {
	t.logger.With().Debug("on layer", last)
	defer t.evict(ctx)
	if last.After(t.last) {
		t.last = last
		lastLayer.Set(float64(t.last))
	}
	if err := t.drainRetriable(); err != nil {
		return nil
	}
	for process := t.processed.Add(1); !process.After(t.last); process = process.Add(1) {
		if process.FirstInEpoch() {
			if err := t.loadAtxs(process.GetEpoch()); err != nil {
				return err
			}
		}
		layer := t.layer(process)
		for _, block := range layer.blocks {
			t.updateRefHeight(layer, block)
		}
		prev := t.layer(process.Sub(1))
		layer.verifying.goodUncounted = layer.verifying.goodUncounted.Add(prev.verifying.goodUncounted)
		t.processed = process
		processedLayer.Set(float64(t.processed))

		if t.isFull {
			t.full.countDelayed(t.logger, process)
			t.full.counted = process
		}
		for _, ballot := range t.ballots[process] {
			if err := t.countBallot(t.logger, ballot); err != nil {
				if errors.Is(err, errBeaconUnavailable) {
					t.retryLater(ballot)
				} else {
					return err
				}
			}
		}

		if err := t.loadBlocksData(process); err != nil {
			return err
		}
		if err := t.loadBallots(process); err != nil {
			return err
		}

		layer.prevOpinion = &prev.opinion
		layer.computeOpinion(t.Hdist, t.last)
		t.logger.With().Debug("initial local opinion",
			layer.lid,
			log.Stringer("local opinion", layer.opinion))

		// terminate layer that falls out of the zdist window and wasn't terminated
		// by any other component
		if process.After(types.LayerID(t.Zdist)) {
			terminated := process.Sub(t.Zdist)
			if terminated.After(t.evicted) && !t.layer(terminated).hareTerminated {
				t.onHareOutput(terminated, types.EmptyBlockID)
			}
		}
	}
	t.verifyLayers()
	return nil
}

func (t *turtle) switchModes(logger log.Log) {
	t.isFull = !t.isFull
	if t.isFull {
		modeGauge.Set(1)
	} else {
		modeGauge.Set(0)
	}
	logger.With().Debug("switching tortoise mode",
		log.Uint32("hdist", t.Hdist),
		log.Stringer("processed_layer", t.processed),
		log.Stringer("verified_layer", t.verified),
		log.Bool("is full", t.isFull),
	)
}

func (t *turtle) countBallot(logger log.Log, ballot *ballotInfo) error {
	bad, err := t.compareBeacons(t.logger, ballot.id, ballot.layer, ballot.reference.beacon)
	if err != nil {
		return fmt.Errorf("%w: %s", errBeaconUnavailable, err.Error())
	}
	ballot.conditions.badBeacon = bad
	t.verifying.countBallot(logger, ballot)
	if !ballot.layer.After(t.full.counted) {
		t.full.countBallot(logger, ballot)
	}
	return nil
}

func (t *turtle) verifyLayers() {
	var (
		logger = t.logger.WithFields(
			log.Stringer("last layer", t.last),
		)
		verified = maxLayer(t.evicted, types.GetEffectiveGenesis())
	)

	if t.changedOpinion.min != 0 && !withinDistance(t.Hdist, t.changedOpinion.max, t.last) {
		logger.With().Debug("changed opinion outside hdist", log.Stringer("from", t.changedOpinion.min), log.Stringer("to", t.changedOpinion.max))
		t.onOpinionChange(t.changedOpinion.min)
		t.changedOpinion.min = types.LayerID(0)
		t.changedOpinion.max = types.LayerID(0)
	}

	for target := t.evicted.Add(1); target.Before(t.processed); target = target.Add(1) {
		success := t.verifying.verify(logger, target)
		if success && t.isFull {
			t.switchModes(logger)
		}
		if !success && (t.isFull || !withinDistance(t.Hdist, target, t.last)) {
			if !t.isFull {
				t.switchModes(logger)
				for counted := maxLayer(t.full.counted.Add(1), t.evicted.Add(1)); !counted.After(t.processed); counted = counted.Add(1) {
					for _, ballot := range t.ballots[counted] {
						t.full.countBallot(logger, ballot)
					}
					t.full.countDelayed(logger, counted)
					t.full.counted = counted
				}
			}
			success = t.full.verify(logger, target)
		}
		if !success {
			break
		}
		verified = target
		for _, block := range t.layers[target].blocks {
			if block.emitted == block.validity {
				continue
			}
			// record range of layers where opinion has changed.
			// once those layers fall out of hdist window - opinion can be recomputed
			if block.validity != block.hare || (block.emitted != block.validity && block.emitted != abstain) {
				if target.After(t.changedOpinion.max) {
					t.changedOpinion.max = target
				}
				if t.changedOpinion.min == 0 || target.Before(t.changedOpinion.min) {
					t.changedOpinion.min = target
				}
			}
			if block.validity == abstain {
				logger.With().Fatal("bug: layer should not be verified if there is an undecided block", target, block.id)
			}
			logger.With().Debug("update validity", block.layer, block.id,
				log.Stringer("validity", block.validity),
				log.Stringer("hare", block.hare),
				log.Stringer("emitted", block.emitted),
			)
			if t.updated == nil {
				t.updated = map[types.LayerID]map[types.BlockID]bool{}
			}
			if _, ok := t.updated[target]; !ok {
				t.updated[target] = map[types.BlockID]bool{}
			}
			t.updated[target][block.id] = block.validity == support
			block.emitted = block.validity
		}
	}
	t.verified = verified
	verifiedLayer.Set(float64(t.verified))
}

// loadBlocksData loads blocks, hare output and contextual validity.
func (t *turtle) loadBlocksData(lid types.LayerID) error {
	blocks, err := blocks.Layer(t.cdb, lid)
	if err != nil {
		return fmt.Errorf("read blocks for layer %s: %w", lid, err)
	}
	for _, block := range blocks {
		t.onBlock(block.Header())
	}
	if err := t.loadHare(lid); err != nil {
		return err
	}
	return t.loadContextualValidity(lid)
}

func (t *turtle) loadHare(lid types.LayerID) error {
	output, err := certificates.GetHareOutput(t.cdb, lid)
	if err == nil {
		t.onHareOutput(lid, output)
		return nil
	}
	if errors.Is(err, sql.ErrNotFound) {
		t.logger.With().Debug("hare output for layer is not found", lid)
		return nil
	}
	return fmt.Errorf("get hare output %s: %w", lid, err)
}

func (t *turtle) loadContextualValidity(lid types.LayerID) error {
	// validities will be available only during rerun or
	// if they are synced from peers
	for _, block := range t.layer(lid).blocks {
		valid, err := blocks.IsValid(t.cdb, block.id)
		if err != nil {
			if !errors.Is(err, blocks.ErrValidityNotDecided) {
				return err
			}
		} else if valid {
			block.validity = support
		} else {
			block.validity = against
		}
	}
	return nil
}

// loadAtxs and compute reference height.
func (t *turtle) loadAtxs(epoch types.EpochID) error {
	var heights []uint64
	if err := t.cdb.IterateEpochATXHeaders(epoch, func(header *types.ActivationTxHeader) bool {
		t.onAtx(header)
		heights = append(heights, header.TickHeight())
		return true
	}); err != nil {
		return fmt.Errorf("computing epoch data for %d: %w", epoch, err)
	}
	einfo := t.epoch(epoch)
	einfo.height = getMedian(heights)
	return nil
}

func (t *turtle) loadBallots(lid types.LayerID) error {
	blts, err := ballots.Layer(t.cdb, lid)
	if err != nil {
		return fmt.Errorf("read ballots for layer %s: %w", lid, err)
	}

	for _, ballot := range blts {
		if err := t.onBallot(ballot); err != nil {
			t.logger.With().Error("failed to add ballot to the state", log.Err(err), log.Inline(ballot))
		}
	}
	return nil
}

func (t *turtle) onBlock(header types.BlockHeader) {
	if !header.Layer.After(t.evicted) {
		return
	}

	if binfo := t.state.getBlock(header); binfo != nil {
		binfo.data = true
		return
	}
	t.logger.With().Debug("on data block", log.Inline(&header))

	binfo := newBlockInfo(header)
	binfo.data = true
	t.addBlock(binfo)
}

func (t *turtle) addBlock(binfo *blockInfo) {
	start := time.Now()
	t.state.addBlock(binfo)
	t.full.countForLateBlock(binfo)
	addBlockDuration.Observe(float64(time.Since(start).Nanoseconds()))
}

func (t *turtle) onHareOutput(lid types.LayerID, bid types.BlockID) {
	start := time.Now()
	if !lid.After(t.evicted) {
		return
	}
	t.logger.With().Debug("on hare output", lid, bid, log.Bool("empty", bid == types.EmptyBlockID))
	var (
		layer    = t.state.layer(lid)
		previous types.BlockID
		exists   bool
	)
	layer.hareTerminated = true
	for i := range layer.blocks {
		block := layer.blocks[i]
		if block.hare == support {
			previous = layer.blocks[i].id
			exists = true
		}
		if block.id == bid {
			block.hare = support
		} else {
			block.hare = against
		}
	}
	if exists && previous == bid {
		return
	}
	if !lid.After(t.processed) && withinDistance(t.Config.Hdist, lid, t.last) {
		t.logger.With().Debug("local opinion changed within hdist",
			lid,
			log.Stringer("verified", t.verified),
			log.Stringer("previous", previous),
			log.Stringer("new", bid),
		)
		t.onOpinionChange(lid)
	}
	addHareOutput.Observe(float64(time.Since(start).Nanoseconds()))
}

func (t *turtle) onOpinionChange(lid types.LayerID) {
	for recompute := lid; !recompute.After(t.processed); recompute = recompute.Add(1) {
		layer := t.layer(recompute)
		layer.computeOpinion(t.Hdist, t.last)
		t.logger.With().Debug("computed local opinion",
			layer.lid,
			log.Stringer("local opinion", layer.opinion))
	}
	t.verifying.resetWeights(lid)
	for target := lid.Add(1); !target.After(t.processed); target = target.Add(1) {
		t.verifying.countVotes(t.logger, t.ballots[target])
	}
}

func (t *turtle) onAtx(atx *types.ActivationTxHeader) {
	start := time.Now()
	epoch := t.epoch(atx.TargetEpoch())
	if _, exist := epoch.atxs[atx.ID]; !exist {
		t.logger.With().Debug("on atx",
			log.Stringer("id", atx.ID),
			log.Uint32("epoch", uint32(atx.TargetEpoch())),
			log.Uint64("weight", atx.GetWeight()),
		)
		epoch.atxs[atx.ID] = atx.GetWeight()
		if atx.GetWeight() > math.MaxInt64 {
			// atx weight is not expected to overflow int64
			t.logger.With().Fatal("fixme: atx size overflows int64", log.Uint64("weight", atx.GetWeight()))
		}
		epoch.weight = epoch.weight.Add(fixed.New64(int64(atx.GetWeight())))
	}
	if atx.TargetEpoch() == t.last.GetEpoch() {
		t.localThreshold = epoch.weight.
			Div(fixed.New(localThresholdFraction)).
			Div(fixed.New64(int64(types.GetLayersPerEpoch())))
	}
	addAtxDuration.Observe(float64(time.Since(start).Nanoseconds()))
}

func (t *turtle) decodeBallot(ballot *types.Ballot) (*ballotInfo, types.LayerID, error) {
	start := time.Now()

	if !ballot.Layer.After(t.evicted) {
		return nil, 0, nil
	}
	if _, exist := t.state.ballotRefs[ballot.ID()]; exist {
		return nil, 0, nil
	}

	t.logger.With().Debug("on ballot",
		log.Inline(ballot),
		log.Uint32("processed", t.processed.Uint32()),
	)

	var (
		base    *ballotInfo
		refinfo *referenceInfo
	)

	if ballot.Votes.Base == types.EmptyBallotID {
		base = &ballotInfo{layer: types.GetEffectiveGenesis()}
	} else {
		base = t.state.ballotRefs[ballot.Votes.Base]
		if base == nil {
			t.logger.With().Warning("base ballot not in state",
				log.Stringer("base", ballot.Votes.Base),
			)
			return nil, 0, nil
		}
	}
	if !base.layer.Before(ballot.Layer) {
		return nil, 0, fmt.Errorf("votes for ballot (%s/%s) should be encoded with base ballot (%s/%s) from previous layers",
			ballot.Layer, ballot.ID(), base.layer, base.id)
	}

	if ballot.EpochData != nil {
		beacon := ballot.EpochData.Beacon
		height, err := getBallotHeight(t.cdb, ballot)
		if err != nil {
			return nil, 0, err
		}
		refweight, err := putil.ComputeWeightPerEligibility(t.cdb, ballot, t.LayerSize, types.GetLayersPerEpoch())
		if err != nil {
			return nil, 0, err
		}
		refinfo = &referenceInfo{
			height: height,
			beacon: beacon,
			weight: refweight,
		}
	} else {
		ref, exists := t.state.ballotRefs[ballot.RefBallot]
		if !exists {
			t.logger.With().Warning("ref ballot not in state",
				log.Stringer("ref", ballot.RefBallot),
			)
			return nil, 0, nil
		}
		if ref.reference == nil {
			return nil, 0, fmt.Errorf("ballot %s is not a reference ballot", ballot.RefBallot)
		}
		refinfo = ref.reference
	}

	binfo := &ballotInfo{
		id: ballot.ID(),
		base: baseInfo{
			id:    base.id,
			layer: base.layer,
		},
		reference: refinfo,
		layer:     ballot.Layer,
	}

	if !ballot.IsMalicious() {
		binfo.weight = fixed.DivUint64(
			refinfo.weight.Num().Uint64(),
			refinfo.weight.Denom().Uint64(),
		).Mul(fixed.New(len(ballot.EligibilityProofs)))
	} else {
		binfo.malicious = true
		t.logger.With().Warning("ballot from malicious identity will have zeroed weight", ballot.Layer, ballot.ID())
	}

	t.logger.With().Debug("computed weight and height for ballot",
		ballot.ID(),
		log.Stringer("weight", binfo.weight),
		log.Uint64("height", refinfo.height),
		log.Uint32("lid", ballot.Layer.Uint32()),
	)

	votes, min, err := decodeVotes(t.evicted, binfo.layer, base, ballot.Votes)
	if err != nil {
		return nil, 0, err
	}
	binfo.votes = votes
	t.logger.With().Debug("decoded exceptions",
		binfo.id, binfo.layer,
		log.Stringer("opinion", binfo.opinion()),
	)
	decodeBallotDuration.Observe(float64(time.Since(start).Nanoseconds()))
	return binfo, min, nil
}

func (t *turtle) storeBallot(ballot *ballotInfo, min types.LayerID) {
	if !ballot.layer.After(t.evicted) {
		return
	}

	t.state.addBallot(ballot)
	for current := ballot.votes.tail; current != nil && !current.lid.Before(min); current = current.prev {
		for i, block := range current.supported {
			existing := t.getBlock(block.header())
			if existing != nil {
				current.supported[i] = existing
			} else {
				t.addBlock(block)
			}
		}
	}
	if !ballot.layer.After(t.processed) {
		if err := t.countBallot(t.logger, ballot); err != nil {
			if errors.Is(err, errBeaconUnavailable) {
				t.retryLater(ballot)
			} else {
				t.logger.Panic("unexpected error in counting ballots", log.Err(err))
			}
		}
	}
}

func (t *turtle) onBallot(ballot *types.Ballot) error {
	decoded, min, err := t.decodeBallot(ballot)
	if decoded == nil || err != nil {
		return err
	}
	t.storeBallot(decoded, min)
	return nil
}

func (t *turtle) compareBeacons(logger log.Log, bid types.BallotID, layerID types.LayerID, beacon types.Beacon) (bool, error) {
	epochBeacon, err := t.beacons.GetBeacon(layerID.GetEpoch())
	if err != nil {
		return false, err
	}
	if beacon != epochBeacon {
		logger.With().Debug("ballot has different beacon",
			layerID,
			bid,
			log.String("ballot_beacon", beacon.ShortString()),
			log.String("epoch_beacon", epochBeacon.ShortString()))
		return true, nil
	}
	return false, nil
}

func (t *turtle) retryLater(ballot *ballotInfo) {
	t.retriable.PushBack(ballot)
}

func (t *turtle) drainRetriable() error {
	for front := t.retriable.Front(); front != nil; {
		if err := t.countBallot(t.logger, front.Value.(*ballotInfo)); err != nil {
			// if beacon is still unavailable - exit and wait for the next call
			// to drain this queue
			if errors.Is(err, errBeaconUnavailable) {
				return nil
			}
			return err
		}
		next := front.Next()
		t.retriable.Remove(front)
		front = next
	}
	return nil
}

func (t *turtle) checkDrained() error {
	if lth := t.retriable.Len(); lth != 0 {
		return fmt.Errorf("all ballots from processed layers (%d) must be counted before encoding votes", lth)
	}
	return nil
}

func withinDistance(dist uint32, lid, last types.LayerID) bool {
	// layer + distance > last
	return lid.Add(dist).After(last)
}

func getLocalVote(config Config, verified, last types.LayerID, block *blockInfo) (sign, voteReason) {
	if withinDistance(config.Hdist, block.layer, last) {
		return block.hare, reasonHareOutput
	}
	if block.layer.After(verified) {
		return abstain, reasonValidity
	}
	return block.validity, reasonValidity
}

func minLayer(i, j types.LayerID) types.LayerID {
	if i < j {
		return i
	}
	return j
}
