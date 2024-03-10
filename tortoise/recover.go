package tortoise

import (
	"context"
	"errors"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/beacons"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/certificates"
	"github.com/spacemeshos/go-spacemesh/sql/identities"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
)

// Recover tortoise state from database.
func Recover(
	ctx context.Context,
	db sql.Executor,
	atxdata *atxsdata.Data,
	current types.LayerID,
	opts ...Opt,
) (*Tortoise, error) {
	trtl, err := New(atxdata, opts...)
	if err != nil {
		return nil, err
	}

	last, err := ballots.LatestLayer(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load latest known layer: %w", err)
	}
	applied, err := layers.GetLastApplied(db)
	if err != nil {
		return nil, fmt.Errorf("get last applied: %w", err)
	}

	start := types.GetEffectiveGenesis() + 1
	if applied > types.LayerID(trtl.cfg.WindowSize) {
		// we want to emulate the same condition as during genesis with one difference.
		// genesis starts with zero opinion (aggregated hash) - see computeOpinion method.
		// but in this case first processed layer should use non-zero opinion of the the previous layer.

		window := applied - types.LayerID(trtl.cfg.WindowSize)
		// we start tallying votes from the first layer of the epoch to guarantee that we load reference ballots.
		// reference ballots track beacon and eligibilities
		window = window.GetEpoch().FirstLayer()
		if window > start {
			prev, err1 := layers.GetAggregatedHash(db, window-1)
			opinion, err2 := layers.GetAggregatedHash(db, window)
			if err1 == nil && err2 == nil {
				// tortoise will need reference to previous layer
				trtl.RecoverFrom(window, opinion, prev)
				start = window
			}
		}
	}

	malicious, err := identities.GetMalicious(db)
	if err != nil {
		return nil, fmt.Errorf("recover malicious %w", err)
	}
	for _, id := range malicious {
		trtl.OnMalfeasance(id)
	}

	if types.GetEffectiveGenesis() != types.FirstEffectiveGenesis() {
		// need to load the golden atxs after a checkpoint recovery
		if err := recoverEpoch(types.GetEffectiveGenesis().Add(1).GetEpoch(), trtl, db, atxdata); err != nil {
			return nil, err
		}
	}

	valid, err := blocks.LastValid(db)
	if err != nil && !errors.Is(err, sql.ErrNotFound) {
		return nil, fmt.Errorf("get last valid: %w", err)
	}
	if err == nil {
		trtl.UpdateVerified(valid)
	}
	trtl.UpdateLastLayer(last)

	for lid := start; !lid.After(last); lid = lid.Add(1) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if err := RecoverLayer(ctx, trtl, db, atxdata, lid, trtl.OnRecoveredBallot); err != nil {
			return nil, fmt.Errorf("failed to load tortoise state at layer %d: %w", lid, err)
		}
	}

	// load activations from future epochs that are not yet referenced by the ballots
	atxsEpoch, err := atxs.LatestEpoch(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load latest epoch: %w", err)
	}
	atxsEpoch++ // recoverEpoch expects target epoch
	if last.GetEpoch() != atxsEpoch {
		for eid := last.GetEpoch() + 1; eid <= atxsEpoch; eid++ {
			if err := recoverEpoch(eid, trtl, db, atxdata); err != nil {
				return nil, err
			}
		}
	}

	last = min(last, current)
	if last < start {
		return trtl, nil
	}
	trtl.TallyVotes(ctx, last)
	// find topmost layer that was already applied with same result
	// and reset pending so that result for that layer is not returned
	for prev := valid; prev >= start; prev-- {
		opinion, err := layers.GetAggregatedHash(db, prev)
		if err == nil && opinion != types.EmptyLayerHash {
			if trtl.OnApplied(prev, opinion) {
				break
			}
		}
		if err != nil && !errors.Is(err, sql.ErrNotFound) {
			return nil, fmt.Errorf("check opinion %w", err)
		}
	}
	return trtl, nil
}

func recoverEpoch(target types.EpochID, trtl *Tortoise, db sql.Executor, atxdata *atxsdata.Data) error {
	atxdata.IterateInEpoch(target, func(id types.ATXID, atx *atxsdata.ATX) {
		trtl.OnAtx(target, id, atx)
	})

	beacon, err := beacons.Get(db, target)
	if err == nil && beacon != types.EmptyBeacon {
		trtl.OnBeacon(target, beacon)
	}
	return nil
}

type ballotFunc func(*types.BallotTortoiseData)

func RecoverLayer(
	ctx context.Context,
	trtl *Tortoise,
	db sql.Executor,
	atxdata *atxsdata.Data,
	lid types.LayerID,
	onBallot ballotFunc,
) error {
	if lid.FirstInEpoch() {
		if err := recoverEpoch(lid.GetEpoch(), trtl, db, atxdata); err != nil {
			return err
		}
	}
	blocksrst, err := blocks.Layer(db, lid)
	if err != nil {
		return err
	}

	results := map[types.BlockHeader]bool{}
	for _, block := range blocksrst {
		valid, err := blocks.IsValid(db, block.ID())
		if err != nil && errors.Is(err, sql.ErrNotFound) {
			return err
		}
		results[block.ToVote()] = valid
	}
	var hareResult *types.BlockID

	// tortoise votes according to the hare only within hdist (protocol parameter).
	// also node is free to prune certificates outside hdist to minimize space usage.
	if trtl.WithinHdist(lid) {
		hare, err := certificates.GetHareOutput(db, lid)
		if err != nil && !errors.Is(err, sql.ErrNotFound) {
			return err
		}
		if err == nil {
			hareResult = &hare
		}
	}
	trtl.OnRecoveredBlocks(lid, results, hareResult)

	// NOTE(dshulyak) we loaded information about malicious identities earlier.
	if err := ballots.IterateForTortoise(db, lid, func(
		id types.BallotID,
		atxid types.ATXID,
		node types.NodeID,
		eligibilities uint32,
		beacon types.Beacon,
		totalEligibilities uint32,
		opinion types.Opinion,
	) bool {
		data := &types.BallotTortoiseData{
			ID:            id,
			Smesher:       node,
			Layer:         lid,
			AtxID:         atxid,
			Eligibilities: eligibilities,
			Opinion:       opinion,
			EpochData: &types.ReferenceData{
				Beacon:        beacon,
				Eligibilities: totalEligibilities,
			},
		}
		onBallot(data)
		return true
	}); err != nil {
		return fmt.Errorf("iterate ballots: %w", err)
	}
	coin, err := layers.GetWeakCoin(db, lid)
	if err != nil && !errors.Is(err, sql.ErrNotFound) {
		return err
	} else if err == nil {
		trtl.OnWeakCoin(lid, coin)
	}
	return nil
}
