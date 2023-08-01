package hare3alt

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/hare/eligibility"
	"github.com/spacemeshos/go-spacemesh/hare/eligibility/config"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/spacemeshos/go-spacemesh/p2p"
	pmocks "github.com/spacemeshos/go-spacemesh/p2p/pubsub/mocks"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/beacons"
	smocks "github.com/spacemeshos/go-spacemesh/system/mocks"
	"github.com/spacemeshos/go-spacemesh/timesync"
)

const layersPerEpoch = 4

func TestMain(m *testing.M) {
	types.SetLayersPerEpoch(layersPerEpoch)
	res := m.Run()
	os.Exit(res)
}

func testHare(tb testing.TB, n int) {
	tb.Helper()
	now := time.Now()
	ctrl := gomock.NewController(tb)
	wall := clock.NewMock()
	wall.Set(now)
	layerDuration := 2 * time.Minute
	nodeclock, err := timesync.NewClock(
		timesync.WithLogger(logtest.New(tb)),
		timesync.WithClock(wall),
		timesync.WithGenesisTime(now),
		timesync.WithLayerDuration(layerDuration),
		timesync.WithTickInterval(layerDuration),
	)
	require.NoError(tb, err)
	syncer := smocks.NewMockSyncStateProvider(ctrl)
	syncer.EXPECT().IsSynced(gomock.Any()).Return(true).AnyTimes()

	verifier, err := signing.NewEdVerifier()
	require.NoError(tb, err)
	vrfverifier := signing.NewVRFVerifier()
	rng := rand.New(rand.NewSource(1001))
	hares := make([]*Hare, n)
	tb.Cleanup(func() {
		for _, hr := range hares {
			hr.Stop()
		}
	})
	pubs := pmocks.NewMockPublishSubsciber(ctrl)
	pubs.EXPECT().Register(gomock.Any(), gomock.Any()).AnyTimes()
	pubs.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, pid p2p.Peer, msg []byte) {
		for _, hr := range hares {
			require.NoError(tb, hr.handler(ctx, pid, msg))
		}
	}).AnyTimes()
	signers := make([]*signing.EdSigner, n)
	for i := range signers {
		signer, err := signing.NewEdSigner(signing.WithKeyFromRand(rng))
		require.NoError(tb, err)
		signers[i] = signer
	}
	genesis := types.GetEffectiveGenesis()
	vatxs := make([]*types.VerifiedActivationTx, n)
	for i := range vatxs {
		atx := &types.ActivationTx{}
		atx.NumUnits = 10
		atx.PublishEpoch = genesis.GetEpoch()
		atx.SmesherID = signers[i].NodeID()
		id := types.ATXID{}
		rng.Read(id[:])
		atx.SetID(id)
		atx.SetEffectiveNumUnits(atx.NumUnits)
		atx.SetReceived(now)
		nonce := types.VRFPostIndex(rng.Uint64())
		atx.VRFNonce = &nonce
		verified, err := atx.Verify(0, 100)
		require.NoError(tb, err)
		vatxs[i] = verified
	}
	for i := 0; i < n; i++ {
		log := logtest.New(tb).Named(strconv.Itoa(i))
		db := datastore.NewCachedDB(sql.InMemory(), log)
		require.NoError(tb, beacons.Add(db, types.GetEffectiveGenesis().GetEpoch()+1, types.Beacon{1, 1, 1}))
		beaconget := smocks.NewMockBeaconGetter(ctrl)
		beaconget.EXPECT().GetBeacon(gomock.Any()).DoAndReturn(func(epoch types.EpochID) (types.Beacon, error) {
			return beacons.Get(db, epoch)
		}).AnyTimes()
		vrfsigner, err := signers[i].VRFSigner()
		require.NoError(tb, err)
		or := eligibility.New(beaconget, db, vrfverifier, vrfsigner, layersPerEpoch, config.DefaultConfig(), log)
		hares[i] = New(nodeclock, pubs, db, verifier, signers[i], or, syncer,
			WithLogger(log.Zap()), WithWallclock(wall), WithEnableLayer(genesis))
		hares[i].Start()
	}
	wall.Set(now.Add(2 * layersPerEpoch * layerDuration))
	time.Sleep(time.Minute)
}

func TestHare(t *testing.T) {
	t.Run("one", func(t *testing.T) { testHare(t, 1) })
}
