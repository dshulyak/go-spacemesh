package atxsdata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
	"github.com/spacemeshos/go-spacemesh/sql/mocks"
)

type gatxOpt func(*types.ActivationTx)

func optUnits(u uint32) gatxOpt {
	return func(tx *types.ActivationTx) {
		tx.NumUnits = u
		tx.SetEffectiveNumUnits(u)
	}
}

func gatx(
	id types.ATXID,
	epoch types.EpochID,
	smesher types.NodeID,
	nonce *types.VRFPostIndex,
	opts ...gatxOpt,
) types.VerifiedActivationTx {
	atx := &types.ActivationTx{}
	atx.NumUnits = 1
	atx.PublishEpoch = epoch
	atx.SmesherID = smesher
	atx.SetID(id)
	atx.SetEffectiveNumUnits(atx.NumUnits)
	atx.SetReceived(time.Time{}.Add(1))
	atx.VRFNonce = nonce
	for _, opt := range opts {
		opt(atx)
	}
	verified, err := atx.Verify(0, 100)
	if err != nil {
		panic(err)
	}
	return *verified
}

func TestWarmup(t *testing.T) {
	types.SetLayersPerEpoch(3)
	t.Run("sanity", func(t *testing.T) {
		db := sql.InMemory()
		applied := types.LayerID(10)
		nonce := types.VRFPostIndex(1)
		data := []types.VerifiedActivationTx{
			gatx(types.ATXID{1, 1}, 1, types.NodeID{1}, &nonce),
			gatx(types.ATXID{1, 2}, 1, types.NodeID{2}, &nonce),
			gatx(types.ATXID{2, 1}, 2, types.NodeID{1}, &nonce),
			gatx(types.ATXID{2, 2}, 2, types.NodeID{2}, &nonce),
			gatx(types.ATXID{3, 2}, 3, types.NodeID{2}, &nonce),
			gatx(types.ATXID{3, 3}, 3, types.NodeID{3}, &nonce),
		}
		for i := range data {
			require.NoError(t, atxs.Add(db, &data[i]))
		}
		require.NoError(t, layers.SetApplied(db, applied, types.BlockID{1}))

		c, err := Warm(db, 1)
		require.NoError(t, err)
		for _, atx := range data[2:] {
			require.NotNil(t, c.Get(atx.TargetEpoch(), atx.ID()))
		}
	})
	t.Run("non decreasing weight", func(t *testing.T) {
		db := sql.InMemory()
		applied := types.LayerID(10)
		nonce := types.VRFPostIndex(1)
		publish := types.EpochID(1)
		data := []types.VerifiedActivationTx{
			gatx(types.ATXID{1, 1}, publish, types.NodeID{1}, &nonce),
			gatx(types.ATXID{1, 2}, publish, types.NodeID{2}, &nonce),
			gatx(types.ATXID{2, 1}, publish+1, types.NodeID{1}, &nonce, optUnits(2)),
			gatx(types.ATXID{2, 1, 1}, publish+1, types.NodeID{1}, &nonce, optUnits(5)),
			gatx(types.ATXID{2, 2}, publish+1, types.NodeID{2}, &nonce),
			gatx(types.ATXID{1, 3, 3}, publish+2, types.NodeID{2}, &nonce, optUnits(3)),
			gatx(types.ATXID{3, 2}, publish+2, types.NodeID{1}, &nonce),
			gatx(types.ATXID{3, 3}, publish+2, types.NodeID{2}, &nonce, optUnits(4)),
		}
		for i := range data {
			require.NoError(t, atxs.Add(db, &data[i]))
		}
		require.NoError(t, layers.SetApplied(db, applied, types.BlockID{1}))

		c, err := Warm(db, WithCapacity(4))
		require.NoError(t, err)
		require.EqualValues(t, 200, int(c.NonDecreasingWeight(publish+1)))
		require.EqualValues(t, 600, int(c.NonDecreasingWeight(publish+2)))
		require.EqualValues(t, 500, int(c.NonDecreasingWeight(publish+3)))
	})
	t.Run("no data", func(t *testing.T) {
		c, err := Warm(sql.InMemory(), 1)
		require.NoError(t, err)
		require.NotNil(t, c)
	})
	t.Run("closed db", func(t *testing.T) {
		db := sql.InMemory()
		require.NoError(t, db.Close())
		c, err := Warm(db, 1)
		require.Error(t, err)
		require.Nil(t, c)
	})
	t.Run("missing nonce", func(t *testing.T) {
		db := sql.InMemory()
		data := gatx(types.ATXID{1, 1}, 1, types.NodeID{1}, nil)
		require.NoError(t, atxs.Add(db, &data))
		c, err := Warm(db, 1)
		require.Error(t, err)
		require.Nil(t, c)
	})
	t.Run("db failures", func(t *testing.T) {
		db := sql.InMemory()
		nonce := types.VRFPostIndex(1)
		data := gatx(types.ATXID{1, 1}, 1, types.NodeID{1}, &nonce)
		require.NoError(t, atxs.Add(db, &data))

		exec := mocks.NewMockExecutor(gomock.NewController(t))
		call := 0
		fail := 0
		tx, err := db.Tx(context.Background())
		require.NoError(t, err)
		exec.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(q string, enc sql.Encoder, dec sql.Decoder) (int, error) {
				if call == fail {
					return 0, errors.New("test")
				}
				call++
				return tx.Exec(q, enc, dec)
			}).
			AnyTimes()
		for i := 0; i < 5; i++ {
			c := New()
			require.Error(t, Warmup(exec, c, 1))
			fail++
			call = 0
		}
	})
}
