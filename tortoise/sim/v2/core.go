package sim

import (
	"context"
	"math/rand"

	"github.com/spacemeshos/go-spacemesh/activation"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/system"
)

var _ StateMachine = (*core)(nil)

func newCore(rng *rand.Rand) *core {
	c := &core{
		rng: rng,
	}
	return c
}

// core state machine. Represents single instance of the honest tortoise consensus.
type core struct {
	logger log.Log
	rng    *rand.Rand

	meshdb   *mesh.DB
	atxdb    *activation.DB
	beacons  *beaconStore
	tortoise system.Tortoise

	// generated on setup
	address types.Address
	units   uint32
	signer  signing.Signer

	// set in the first layer of each epoch
	refBallot *types.BallotID

	// set in the last layer of each epoch
	atx types.ATXID
}

// OnEvent receive blocks, atx, input vector, beacon, coinflip and store them.
// Generate atx at the end of each epoch.
// Generate block at the start of every layer.
func (c *core) OnEvent(event Event) []Event {
	switch ev := event.(type) {
	case EventLayerStart:
		// TODO(dshulyak) need to check if this instance is eligible to vote
		// it can also be eligible to vote more than once.
		// should i reuse original miner module for this or it will be too complex?

		base, votes, err := c.tortoise.BaseBlock(context.TODO())
		if err != nil {
			panic(err)
		}
		block := &types.Block{
			MiniBlock: types.MiniBlock{
				BlockHeader: types.BlockHeader{
					LayerIndex:  ev.LayerID,
					ATXID:       c.atx,
					BaseBlock:   base,
					AgainstDiff: votes[0],
					ForDiff:     votes[1],
					NeutralDiff: votes[2],
				},
			},
		}
		if c.refBallot != nil {
			block.RefBlock = (*types.BlockID)(c.refBallot)
		} else {
			_, activeset, err := c.atxdb.GetEpochWeight(ev.LayerID.GetEpoch())
			if err != nil {
				panic(err)
			}
			block.ActiveSet = &activeset
			beacon, err := c.beacons.GetBeacon(ev.LayerID.GetEpoch())
			if err != nil {
				panic(err)
			}
			block.TortoiseBeacon = beacon
		}
		block.Signature = c.signer.Sign(block.Bytes())
		block.Initialize()

		if c.refBallot == nil {
			id := types.BallotID(block.ID())
			c.refBallot = &id
		}
	case EventHareTerminated:

	case EventLayerEnd:
		if ev.LayerID.GetEpoch() == ev.LayerID.Add(1).GetEpoch() {
			return nil
		}
		nipost := types.NIPostChallenge{
			NodeID:     types.NodeID{Key: c.address.Hex()},
			StartTick:  1,
			EndTick:    2,
			PubLayerID: ev.LayerID,
		}
		atx := types.NewActivationTx(nipost, c.address, nil, uint(c.units), nil)

		c.refBallot = nil
		c.atx = atx.ID()

		return []Event{
			EventAtx{Atx: atx},
		}
	case EventBlock:
		c.meshdb.AddBlock(ev.Block)
	case EventAtx:
		c.atxdb.StoreAtx(context.TODO(), ev.Atx.TargetEpoch(), ev.Atx)
	case EventBeacon:
		c.beacons.StoreBeacon(ev.EpochID, ev.Beacon)
	case EventCoinflip:
		c.meshdb.RecordCoinflip(context.TODO(), ev.LayerID, ev.Coinflip)
	case EventLayerVector:
		c.meshdb.SaveLayerInputVectorByID(context.TODO(), ev.LayerID, ev.Vector)
	}
	return nil
}

type beaconStore struct {
	beacons map[types.EpochID][]byte
}

func (b *beaconStore) GetBeacon(eid types.EpochID) ([]byte, error) {
	beacon, exist := b.beacons[eid-1]
	if !exist {
		return nil, database.ErrNotFound
	}
	return beacon, nil
}

func (b *beaconStore) StoreBeacon(eid types.EpochID, beacon []byte) {
	b.beacons[eid] = beacon
}

func newAtxDB(logger log.Log, mdb *mesh.DB, layersPerEpoch uint32) *activation.DB {
	db := database.NewMemDatabase()
	return activation.NewDB(db, nil, nil, mdb, layersPerEpoch, types.ATXID{1}, nil, logger)
}

func newMeshDB(logger log.Log) *mesh.DB {
	return mesh.NewMemMeshDB(logger)
}
