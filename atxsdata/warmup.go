package atxsdata

import (
	"context"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/identities"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
)

func Warm(db *sql.Database, keep types.EpochID) (*Data, error) {
	cache := New()
	tx, err := db.Tx(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Release()
	if err := Warmup(tx, cache, keep); err != nil {
		return nil, fmt.Errorf("warmup %w", err)
	}
	return cache, nil
}

func Warmup(db sql.Executor, cache *Data, keep types.EpochID) error {
	latest, err := atxs.LatestEpoch(db)
	if err != nil {
		return err
	}
	applied, err := layers.GetLastApplied(db)
	if err != nil {
		return err
	}
	var evict types.EpochID
	if applied.GetEpoch() > keep {
		evict = applied.GetEpoch() - keep - 1
	}
	cache.EvictEpoch(evict)

	var (
		ierr      error
		prevEpoch types.EpochID
		prevNode  types.NodeID
		largest   *types.ATXID
	)
	if err := atxs.IterateAtxsData(db, cache.Evicted(), latest,
		func(
			id types.ATXID,
			node types.NodeID,
			epoch types.EpochID,
			coinbase types.Address,
			weight,
			base,
			height uint64,
		) bool {
			target := epoch + 1
			nonce, err := atxs.VRFNonce(db, node, target)
			if err != nil {
				ierr = fmt.Errorf("missing nonce %w", err)
				return false
			}
			malicious, err := identities.IsMalicious(db, node)
			if err != nil {
				ierr = err
				return false
			}
			atx := &ATX{
				Node:       node,
				Coinbase:   coinbase,
				Weight:     weight,
				BaseHeight: base,
				Height:     height,
				Nonce:      nonce,
				malicious:  malicious,
			}
			if prevEpoch == epoch && prevNode == node {
				cache.AddWithReplacement(target, id, atx, largest)
			} else {
				// we iterate in the order of largest to smallest weight for equivocating atxs
				cache.AddWithReplacement(target, id, atx, nil)
				largest = &id
			}
			prevEpoch = epoch
			prevNode = node
			return true
		}); err != nil {
		return err
	}
	return ierr
}
