package tortoisebeacon

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/common/util"
	"github.com/spacemeshos/go-spacemesh/database"
	dbMocks "github.com/spacemeshos/go-spacemesh/database/mocks"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	pubsubmocks "github.com/spacemeshos/go-spacemesh/p2p/pubsub/mocks"
	"github.com/spacemeshos/go-spacemesh/signing"
	smocks "github.com/spacemeshos/go-spacemesh/system/mocks"
	"github.com/spacemeshos/go-spacemesh/timesync"
	"github.com/spacemeshos/go-spacemesh/tortoisebeacon/mocks"
	"github.com/spacemeshos/go-spacemesh/tortoisebeacon/weakcoin"
)

func coinValueMock(tb testing.TB, value bool) coin {
	ctrl := gomock.NewController(tb)
	defer ctrl.Finish()

	coinMock := mocks.NewMockcoin(ctrl)
	coinMock.EXPECT().StartEpoch(
		gomock.Any(),
		gomock.AssignableToTypeOf(types.EpochID(0)),
		gomock.AssignableToTypeOf(weakcoin.UnitAllowances{}),
	).AnyTimes()
	coinMock.EXPECT().FinishEpoch(gomock.Any(), gomock.AssignableToTypeOf(types.EpochID(0))).AnyTimes()
	coinMock.EXPECT().StartRound(gomock.Any(), gomock.AssignableToTypeOf(types.RoundID(0))).
		AnyTimes().Return(nil)
	coinMock.EXPECT().FinishRound(gomock.Any()).AnyTimes()
	coinMock.EXPECT().Get(
		gomock.Any(),
		gomock.AssignableToTypeOf(types.EpochID(0)),
		gomock.AssignableToTypeOf(types.RoundID(0)),
	).AnyTimes().Return(value)
	return coinMock
}

func setUpTortoiseBeacon(t *testing.T, mockEpochWeight uint64, hasATX bool) (*TortoiseBeacon, *timesync.TimeClock) {
	conf := UnitTestConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockactivationDB(ctrl)
	if hasATX {
		mockDB.EXPECT().GetNodeAtxIDForEpoch(gomock.Any(), gomock.Any()).Return(types.ATXID{}, nil).MaxTimes(3)
	} else {
		mockDB.EXPECT().GetNodeAtxIDForEpoch(gomock.Any(), gomock.Any()).Return(types.ATXID{}, database.ErrNotFound).MaxTimes(3)
	}
	// epoch 2, 3, and 4 (since we waited til epoch 3 in each test)
	mockDB.EXPECT().GetEpochWeight(gomock.Any()).Return(mockEpochWeight, nil, nil).MaxTimes(3)
	mwc := coinValueMock(t, true)

	types.SetLayersPerEpoch(3)
	logger := logtest.New(t).WithName("TortoiseBeacon")
	genesisTime := time.Now().Add(100 * time.Millisecond)
	ld := 100 * time.Millisecond
	clock := timesync.NewClock(timesync.RealClock{}, ld, genesisTime, logtest.New(t).WithName("clock"))
	clock.StartNotifying()

	edSgn := signing.NewEdSigner()
	edPubkey := edSgn.PublicKey()
	vrfSigner, vrfPub, err := signing.NewVRFSigner(edSgn.Sign(edPubkey.Bytes()))
	require.NoError(t, err)

	publisher := newPublisher(t)
	minerID := types.NodeID{Key: edPubkey.String(), VRFPublicKey: vrfPub}
	ms := smocks.NewMockSyncStateProvider(ctrl)
	ms.EXPECT().IsSynced(gomock.Any()).Return(true).AnyTimes()
	tb := New(conf, minerID, publisher, mockDB, edSgn, signing.NewEDVerifier(), vrfSigner, signing.VRFVerifier{}, mwc, database.NewMemDatabase(), clock, logger)
	require.NotNil(t, tb)
	tb.SetSyncState(ms)
	tb.setMetricsRegistry(prometheus.NewPedanticRegistry())
	return tb, clock
}

func newPublisher(tb testing.TB) pubsub.Publisher {
	tb.Helper()
	ctrl := gomock.NewController(tb)
	defer ctrl.Finish()

	publisher := pubsubmocks.NewMockPublisher(ctrl)
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
	return publisher
}

func TestTortoiseBeacon(t *testing.T) {
	tb, clock := setUpTortoiseBeacon(t, uint64(10), true)
	epoch := types.EpochID(3)
	require.NoError(t, tb.Start(context.TODO()))

	t.Logf("Awaiting epoch %v", epoch)
	awaitEpoch(clock, epoch)

	v, err := tb.GetBeacon(epoch)
	require.NoError(t, err)

	expected := "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	assert.Equal(t, types.HexToBeacon(expected), v)

	tb.Close()
	clock.Close()
}

func TestTortoiseBeaconZeroWeightEpoch(t *testing.T) {
	tb, clock := setUpTortoiseBeacon(t, uint64(0), true)
	epoch := types.EpochID(3)
	require.NoError(t, tb.Start(context.TODO()))

	t.Logf("Awaiting epoch %v", epoch)
	awaitEpoch(clock, epoch)

	v, err := tb.GetBeacon(epoch)
	assert.Equal(t, ErrBeaconNotCalculated, err)
	assert.Equal(t, types.EmptyBeacon, v)

	tb.Close()
	clock.Close()
}

func TestTortoiseBeaconNoATXInPreviousEpoch(t *testing.T) {
	tb, clock := setUpTortoiseBeacon(t, uint64(0), false)
	epoch := types.EpochID(3)
	require.NoError(t, tb.Start(context.TODO()))

	t.Logf("Awaiting epoch %v", epoch)
	awaitEpoch(clock, epoch)

	v, err := tb.GetBeacon(epoch)
	assert.Equal(t, ErrBeaconNotCalculated, err)
	assert.Equal(t, types.EmptyBeacon, v)

	tb.Close()
	clock.Close()
}

func awaitEpoch(clock *timesync.TimeClock, epoch types.EpochID) {
	layerTicker := clock.Subscribe()

	for layer := range layerTicker {
		// Wait until required epoch passes.
		if layer.GetEpoch() > epoch {
			return
		}
	}
}

func TestTortoiseBeaconWithMetrics(t *testing.T) {
	layersPerEpoch := uint32(3)
	types.SetLayersPerEpoch(layersPerEpoch)

	conf := UnitTestConfig()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDB := mocks.NewMockactivationDB(ctrl)
	mockDB.EXPECT().GetNodeAtxIDForEpoch(gomock.Any(), gomock.Any()).Return(types.ATXID{}, nil).AnyTimes()
	mockDB.EXPECT().GetEpochWeight(gomock.Any()).Return(uint64(10000), nil, nil).AnyTimes()
	atxHeader := &types.ActivationTxHeader{
		NIPostChallenge: types.NIPostChallenge{
			StartTick: 0,
			EndTick:   1,
		},
		NumUnits: 199,
	}
	mockDB.EXPECT().GetAtxHeader(gomock.Any()).Return(atxHeader, nil).AnyTimes()
	mockDB.EXPECT().GetNodeAtxIDForEpoch(gomock.Any(), gomock.Any()).Return(types.ATXID{}, nil).AnyTimes()

	mwc := coinValueMock(t, true)

	logger := logtest.New(t).WithName("TortoiseBeacon")
	clock := clockNeverNotify(t)

	edSgn := signing.NewEdSigner()
	edPubkey := edSgn.PublicKey()
	vrfSigner, vrfPub, err := signing.NewVRFSigner(edSgn.Sign(edPubkey.Bytes()))
	require.NoError(t, err)

	minerID := types.NodeID{Key: edPubkey.String(), VRFPublicKey: vrfPub}
	ms := smocks.NewMockSyncStateProvider(ctrl)
	ms.EXPECT().IsSynced(gomock.Any()).Return(true).AnyTimes()
	tb := New(conf, minerID, newPublisher(t), mockDB, edSgn, signing.NewEDVerifier(), vrfSigner, signing.VRFVerifier{}, mwc, database.NewMemDatabase(), clock, logger)
	require.NotNil(t, tb)
	tb.SetSyncState(ms)

	require.NoError(t, tb.Start(context.TODO()))
	t.Cleanup(func() {
		tb.Close()
	})

	gLayer := types.GetEffectiveGenesis()
	numCalculatedBeacon := 0
	for layer := types.NewLayerID(1); !layer.After(gLayer); layer = layer.Add(1) {
		tb.handleLayer(context.TODO(), layer)
		thisEpoch := layer.GetEpoch()
		ownWeight := atxHeader.GetWeight()
		if thisEpoch.IsGenesis() {
			ownWeight = 0
		}
		allMetrics, err := prometheus.DefaultGatherer.Gather()
		assert.NoError(t, err)
		for _, m := range allMetrics {
			switch *m.Name {
			case "spacemesh_beacons_beacon_calculated_weight":
				require.Equal(t, 1, len(m.Metric))
				numCalculatedBeacon++
				beaconStr := types.HexToHash32(types.GenesisBeacon).ShortString()
				expected := fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beaconStr, thisEpoch+1, ownWeight)
				assert.Equal(t, expected, m.Metric[0].String())
			case "spacemesh_beacons_beacon_observed_total":
				t.Errorf("genesis ballots do not have ballots")
			case "spacemesh_beacons_beacon_observed_weight":
				t.Errorf("genesis ballots do not have ballots")
			}
		}
	}
	assert.Equal(t, int(gLayer.Uint32()), numCalculatedBeacon)

	epoch3Beacon := "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	epoch := types.EpochID(3)
	finalLayer := types.NewLayerID(layersPerEpoch * uint32(epoch))
	beacon1 := types.RandomBeacon()
	beacon2 := types.RandomBeacon()
	for layer := gLayer.Add(1); layer.Before(finalLayer); layer = layer.Add(1) {
		tb.handleLayer(context.TODO(), layer)
		thisEpoch := layer.GetEpoch()
		tb.recordBeacon(thisEpoch, types.RandomBallotID(), beacon1, 100)
		tb.recordBeacon(thisEpoch, types.RandomBallotID(), beacon2, 200)

		numCalculated := 0
		numObserved := 0
		numObservedWeight := 0
		tb.logger.Info("gathering data from test in epoch %v", thisEpoch)
		allMetrics, err := prometheus.DefaultGatherer.Gather()
		assert.NoError(t, err)
		for _, m := range allMetrics {
			switch *m.Name {
			case "spacemesh_beacons_beacon_calculated_weight":
				require.Equal(t, 1, len(m.Metric))
				numCalculated++
				beaconStr := types.HexToHash32(epoch3Beacon).ShortString()
				expected := fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beaconStr, thisEpoch+1, atxHeader.GetWeight())
				assert.Equal(t, expected, m.Metric[0].String())
			case "spacemesh_beacons_beacon_observed_total":
				tb.logger.Info(m.String())
				require.Equal(t, 2, len(m.Metric))
				numObserved = numObserved + 2
				count := layer.OrdinalInEpoch() + 1
				expected := []string{
					fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beacon1.ShortString(), thisEpoch, count),
					fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beacon2.ShortString(), thisEpoch, count),
				}
				for _, subM := range m.Metric {
					assert.Contains(t, expected, subM.String())
				}
			case "spacemesh_beacons_beacon_observed_weight":
				require.Equal(t, 2, len(m.Metric))
				numObservedWeight = numObservedWeight + 2
				weight := (layer.OrdinalInEpoch() + 1) * 100
				expected := []string{
					fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beacon1.ShortString(), thisEpoch, weight),
					fmt.Sprintf("label:<name:\"beacon\" value:\"%s\" > label:<name:\"epoch\" value:\"%d\" > counter:<value:%d > ", beacon2.ShortString(), thisEpoch, weight*2),
				}
				for _, subM := range m.Metric {
					assert.Contains(t, expected, subM.String())
				}
			}
		}
		if layer.OrdinalInEpoch() == 2 {
			// there should be calculated beacon already by the last layer of the epoch
			assert.Equal(t, 1, numCalculated, layer)
		} else {
			assert.LessOrEqual(t, numCalculated, 1, layer)
		}
		assert.Equal(t, 2, numObserved, layer)
		assert.Equal(t, 2, numObservedWeight, layer)
	}
}

func TestTortoiseBeacon_BeaconsWithDatabase(t *testing.T) {
	t.Parallel()

	tb := &TortoiseBeacon{
		logger:  logtest.New(t).WithName("TortoiseBeacon"),
		beacons: make(map[types.EpochID]types.Beacon),
		db:      database.NewMemDatabase(),
	}
	epoch3 := types.EpochID(3)
	beacon2 := types.RandomBeacon()
	epoch5 := types.EpochID(5)
	beacon4 := types.RandomBeacon()
	err := tb.setBeacon(epoch3-1, beacon2)
	require.NoError(t, err)
	err = tb.setBeacon(epoch5-1, beacon4)
	require.NoError(t, err)

	got, err := tb.GetBeacon(epoch3)
	assert.NoError(t, err)
	assert.Equal(t, beacon2, got)

	got, err = tb.GetBeacon(epoch5)
	assert.NoError(t, err)
	assert.Equal(t, beacon4, got)

	got, err = tb.GetBeacon(epoch5 - 1)
	assert.Equal(t, ErrBeaconNotCalculated, err)
	assert.Equal(t, types.EmptyBeacon, got)

	// clear out the in-memory map
	// the database should still give us values
	tb.mu.Lock()
	tb.beacons = make(map[types.EpochID]types.Beacon)
	tb.mu.Unlock()

	got, err = tb.GetBeacon(epoch3)
	assert.NoError(t, err)
	assert.Equal(t, beacon2, got)

	got, err = tb.GetBeacon(epoch5)
	assert.NoError(t, err)
	assert.Equal(t, beacon4, got)

	got, err = tb.GetBeacon(epoch5 - 1)
	assert.Equal(t, ErrBeaconNotCalculated, err)
	assert.Equal(t, types.EmptyBeacon, got)
}

func TestTortoiseBeacon_BeaconsWithDatabaseFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockDB := dbMocks.NewMockDatabase(ctrl)
	tb := &TortoiseBeacon{
		logger:  logtest.New(t).WithName("TortoiseBeacon"),
		beacons: make(map[types.EpochID]types.Beacon),
		db:      mockDB,
	}
	epoch := types.EpochID(3)
	beacon := types.RandomBeacon()
	mockDB.EXPECT().Put(epoch.ToBytes(), beacon.Bytes()).Return(errUnknown).Times(1)
	err := tb.persistBeacon(epoch, beacon)
	assert.ErrorIs(t, err, errUnknown)

	mockDB.EXPECT().Get(epoch.ToBytes()).Return(nil, errUnknown).Times(1)
	got, errGet := tb.getPersistedBeacon(epoch)
	assert.Equal(t, types.EmptyBeacon, got)
	assert.ErrorIs(t, errGet, errUnknown)
}

func TestTortoiseBeacon_BeaconsCleanupOldEpoch(t *testing.T) {
	t.Parallel()

	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		db:                 database.NewMemDatabase(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: make(map[types.EpochID]map[types.Beacon]*ballotWeight),
	}

	epoch := types.EpochID(5)
	for i := 0; i < numEpochsToKeep; i++ {
		e := epoch + types.EpochID(i)
		err := tb.setBeacon(e, types.RandomBeacon())
		require.NoError(t, err)
		tb.recordBeacon(e, types.RandomBallotID(), types.RandomBeacon(), 10)
		tb.cleanupEpoch(e)
		assert.Equal(t, i+1, len(tb.beacons))
		assert.Equal(t, i+1, len(tb.beaconsFromBallots))
	}
	assert.Equal(t, numEpochsToKeep, len(tb.beacons))
	assert.Equal(t, numEpochsToKeep, len(tb.beaconsFromBallots))

	epoch = epoch + numEpochsToKeep
	err := tb.setBeacon(epoch, types.RandomBeacon())
	require.NoError(t, err)
	tb.recordBeacon(epoch, types.RandomBallotID(), types.RandomBeacon(), 10)
	assert.Equal(t, numEpochsToKeep+1, len(tb.beacons))
	assert.Equal(t, numEpochsToKeep+1, len(tb.beaconsFromBallots))
	tb.cleanupEpoch(epoch)
	assert.Equal(t, numEpochsToKeep, len(tb.beacons))
	assert.Equal(t, numEpochsToKeep, len(tb.beaconsFromBallots))
}

func TestTortoiseBeacon_ReportBeaconFromBallot(t *testing.T) {
	t.Parallel()

	types.SetLayersPerEpoch(3)
	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		config:             UnitTestConfig(),
		db:                 database.NewMemDatabase(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: make(map[types.EpochID]map[types.Beacon]*ballotWeight),
	}
	tb.config.BeaconSyncNumBallots = 3

	epoch := types.EpochID(3)
	beacon1 := types.RandomBeacon()
	beacon2 := types.RandomBeacon()
	tb.ReportBeaconFromBallot(epoch, types.RandomBallotID(), beacon1, 100)
	got, err := tb.GetBeacon(epoch)
	require.Equal(t, ErrBeaconNotCalculated, err)
	require.Equal(t, types.EmptyBeacon, got)
	tb.ReportBeaconFromBallot(epoch, types.RandomBallotID(), beacon2, 100)
	got, err = tb.GetBeacon(epoch)
	require.Equal(t, ErrBeaconNotCalculated, err)
	require.Equal(t, types.EmptyBeacon, got)
	tb.ReportBeaconFromBallot(epoch, types.RandomBallotID(), beacon1, 1)
	got, err = tb.GetBeacon(epoch)
	assert.NoError(t, err)
	assert.Equal(t, beacon1, got)
}

func TestTortoiseBeacon_ReportBeaconFromBallot_SameBallot(t *testing.T) {
	t.Parallel()

	types.SetLayersPerEpoch(3)
	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		config:             UnitTestConfig(),
		db:                 database.NewMemDatabase(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: make(map[types.EpochID]map[types.Beacon]*ballotWeight),
	}
	tb.config.BeaconSyncNumBallots = 2

	epoch := types.EpochID(3)
	beacon1 := types.RandomBeacon()
	beacon2 := types.RandomBeacon()
	ballotID1 := types.RandomBallotID()
	ballotID2 := types.RandomBallotID()
	tb.ReportBeaconFromBallot(epoch, ballotID1, beacon1, 100)
	tb.ReportBeaconFromBallot(epoch, ballotID1, beacon1, 200)
	// same ballotID does not count twice
	got, err := tb.GetBeacon(epoch)
	require.Equal(t, ErrBeaconNotCalculated, err)
	require.Equal(t, types.EmptyBeacon, got)

	tb.ReportBeaconFromBallot(epoch, ballotID2, beacon2, 101)
	got, err = tb.GetBeacon(epoch)
	assert.NoError(t, err)
	assert.Equal(t, beacon2, got)
}

func TestTortoiseBeacon_ensureEpochHasBeacon_BeaconAlreadyCalculated(t *testing.T) {
	t.Parallel()

	epoch := types.EpochID(3)
	beacon := types.RandomBeacon()
	beaconFromBlocks := types.RandomBeacon()
	tb := &TortoiseBeacon{
		logger: logtest.New(t).WithName("TortoiseBeacon"),
		config: UnitTestConfig(),
		beacons: map[types.EpochID]types.Beacon{
			epoch - 1: beacon,
		},
		beaconsFromBallots: make(map[types.EpochID]map[types.Beacon]*ballotWeight),
	}
	tb.config.BeaconSyncNumBallots = 2

	got, err := tb.GetBeacon(epoch)
	require.NoError(t, err)
	require.Equal(t, beacon, got)

	tb.ReportBeaconFromBallot(epoch, types.RandomBallotID(), beaconFromBlocks, 100)
	tb.ReportBeaconFromBallot(epoch, types.RandomBallotID(), beaconFromBlocks, 200)

	// should not change the beacon value
	got, err = tb.GetBeacon(epoch)
	require.NoError(t, err)
	require.Equal(t, beacon, got)
}

func TestTortoiseBeacon_findMostWeightedBeaconForEpoch(t *testing.T) {
	t.Parallel()

	types.SetLayersPerEpoch(3)
	beacon1 := types.RandomBeacon()
	beacon2 := types.RandomBeacon()
	beacon3 := types.RandomBeacon()

	beaconsFromBlocks := map[types.Beacon]*ballotWeight{
		beacon1: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}, types.RandomBallotID(): {}},
			weight:  200,
		},
		beacon2: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}},
			weight:  201,
		},
		beacon3: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}},
			weight:  200,
		},
	}
	epoch := types.EpochID(3)
	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		config:             UnitTestConfig(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: map[types.EpochID]map[types.Beacon]*ballotWeight{epoch: beaconsFromBlocks},
	}
	tb.config.BeaconSyncNumBallots = 2
	got := tb.findMostWeightedBeaconForEpoch(epoch)
	assert.Equal(t, beacon2, got)
}

func TestTortoiseBeacon_findMostWeightedBeaconForEpoch_NotEnoughBlocks(t *testing.T) {
	t.Parallel()

	types.SetLayersPerEpoch(3)
	beacon1 := types.RandomBeacon()
	beacon2 := types.RandomBeacon()
	beacon3 := types.RandomBeacon()

	beaconsFromBlocks := map[types.Beacon]*ballotWeight{
		beacon1: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}, types.RandomBallotID(): {}},
			weight:  200,
		},
		beacon2: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}},
			weight:  201,
		},
		beacon3: {
			ballots: map[types.BallotID]struct{}{types.RandomBallotID(): {}},
			weight:  200,
		},
	}
	epoch := types.EpochID(3)
	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		config:             UnitTestConfig(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: map[types.EpochID]map[types.Beacon]*ballotWeight{epoch: beaconsFromBlocks},
	}
	tb.config.BeaconSyncNumBallots = 5
	got := tb.findMostWeightedBeaconForEpoch(epoch)
	assert.Equal(t, types.EmptyBeacon, got)
}

func TestTortoiseBeacon_findMostWeightedBeaconForEpoch_NoBeacon(t *testing.T) {
	t.Parallel()

	types.SetLayersPerEpoch(3)
	tb := &TortoiseBeacon{
		logger:             logtest.New(t).WithName("TortoiseBeacon"),
		config:             UnitTestConfig(),
		beacons:            make(map[types.EpochID]types.Beacon),
		beaconsFromBallots: make(map[types.EpochID]map[types.Beacon]*ballotWeight),
	}
	epoch := types.EpochID(3)
	got := tb.findMostWeightedBeaconForEpoch(epoch)
	assert.Equal(t, types.EmptyBeacon, got)
}

func TestTortoiseBeacon_getProposalChannel(t *testing.T) {
	t.Parallel()

	currentEpoch := types.EpochID(10)

	tt := []struct {
		name            string
		epoch           types.EpochID
		proposalEndTime time.Time
		makeNextChFull  bool
		expected        bool
	}{
		{
			name:     "old epoch",
			epoch:    currentEpoch - 1,
			expected: false,
		},
		{
			name:     "on time",
			epoch:    currentEpoch,
			expected: true,
		},
		{
			name:            "proposal phase ended",
			epoch:           currentEpoch,
			proposalEndTime: time.Now(),
			expected:        false,
		},
		{
			name:     "accept next epoch",
			epoch:    currentEpoch + 1,
			expected: true,
		},
		{
			name:           "next epoch full",
			epoch:          currentEpoch + 1,
			makeNextChFull: true,
			expected:       true,
		},
		{
			name:     "no future epoch beyond next",
			epoch:    currentEpoch + 2,
			expected: false,
		},
	}

	for i, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// don't run these tests in parallel. somehow all cases end up using the same TortoiseBeacon instance
			// t.Parallel()
			tb := TortoiseBeacon{
				logger:                    logtest.New(t).WithName(fmt.Sprintf("TortoiseBeacon-%v", i)),
				proposalChans:             make(map[types.EpochID]chan *proposalMessageWithReceiptData),
				epochInProgress:           currentEpoch,
				proposalPhaseFinishedTime: tc.proposalEndTime,
			}

			if tc.makeNextChFull {
				tb.mu.Lock()
				nextCh := tb.getOrCreateProposalChannel(currentEpoch + 1)
				for i := 0; i < proposalChanCapacity; i++ {
					nextCh <- &proposalMessageWithReceiptData{
						message: ProposalMessage{},
					}
				}
				tb.mu.Unlock()
			}

			ch := tb.getProposalChannel(context.TODO(), tc.epoch)
			if tc.expected {
				assert.NotNil(t, ch)
			} else {
				assert.Nil(t, ch)
			}
		})
	}
}

func TestTortoiseBeacon_votingThreshold(t *testing.T) {
	t.Parallel()

	r := require.New(t)

	tt := []struct {
		name      string
		theta     *big.Rat
		weight    uint64
		threshold *big.Int
	}{
		{
			name:      "Case 1",
			theta:     big.NewRat(1, 2),
			weight:    10,
			threshold: big.NewInt(5),
		},
		{
			name:      "Case 2",
			theta:     big.NewRat(3, 10),
			weight:    10,
			threshold: big.NewInt(3),
		},
		{
			name:      "Case 3",
			theta:     big.NewRat(1, 25000),
			weight:    31744,
			threshold: big.NewInt(1),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tb := TortoiseBeacon{
				logger: logtest.New(t).WithName("TortoiseBeacon"),
				config: Config{},
				theta:  new(big.Float).SetRat(tc.theta),
			}

			threshold := tb.votingThreshold(tc.weight)
			r.EqualValues(tc.threshold, threshold)
		})
	}
}

func TestTortoiseBeacon_atxThresholdFraction(t *testing.T) {
	t.Parallel()

	r := require.New(t)

	theta1, ok := new(big.Float).SetString("0.5")
	r.True(ok)

	theta2, ok := new(big.Float).SetString("0.0")
	r.True(ok)

	tt := []struct {
		name      string
		kappa     uint64
		q         *big.Rat
		w         uint64
		threshold *big.Float
	}{
		{
			name:      "with epoch weight",
			kappa:     40,
			q:         big.NewRat(1, 3),
			w:         60,
			threshold: theta1,
		},
		{
			name:      "zero epoch weight",
			kappa:     40,
			q:         big.NewRat(1, 3),
			w:         0,
			threshold: theta2,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			threshold := atxThresholdFraction(tc.kappa, tc.q, tc.w)
			r.Equal(tc.threshold.String(), threshold.String())
		})
	}
}

func TestTortoiseBeacon_atxThreshold(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	tt := []struct {
		name      string
		kappa     uint64
		q         *big.Rat
		w         uint64
		threshold string
	}{
		{
			name:      "Case 1",
			kappa:     40,
			q:         big.NewRat(1, 3),
			w:         60,
			threshold: "0x80000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:      "Case 2",
			kappa:     400000,
			q:         big.NewRat(1, 3),
			w:         31744,
			threshold: "0xffffddbb63fcd30f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:      "Case 3",
			kappa:     40,
			q:         new(big.Rat).SetFloat64(0.33),
			w:         60,
			threshold: "0x7f8ece00fe541f9f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			threshold := atxThreshold(tc.kappa, tc.q, tc.w)
			r.EqualValues(tc.threshold, fmt.Sprintf("%#x", threshold))
		})
	}
}

func TestTortoiseBeacon_proposalPassesEligibilityThreshold(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	tt := []struct {
		name     string
		kappa    uint64
		q        *big.Rat
		w        uint64
		proposal []byte
		passes   bool
	}{
		{
			name:     "Case 1",
			kappa:    400000,
			q:        big.NewRat(1, 3),
			w:        31744,
			proposal: util.Hex2Bytes("cee191e87d83dc4fbd5e2d40679accf68237b1f09f73f16db5b05ae74b522def9d2ffee56eeb02070277be99a80666ffef9fd4514a51dc37419dd30a791e0f05"),
			passes:   true,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := createProposalChecker(tc.kappa, tc.q, tc.w, logtest.New(t).WithName("proposal checker"))
			passes := checker.IsProposalEligible(tc.proposal)
			r.EqualValues(tc.passes, passes)
		})
	}
}

func TestTortoiseBeacon_buildProposal(t *testing.T) {
	t.Parallel()

	r := require.New(t)

	tt := []struct {
		name   string
		epoch  types.EpochID
		result string
	}{
		{
			name:   "Case 1",
			epoch:  0x12345678,
			result: string(util.Hex2Bytes("000000035442500012345678")),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := buildProposal(tc.epoch, logtest.New(t))
			r.Equal(tc.result, string(result))
		})
	}
}

func TestTortoiseBeacon_signMessage(t *testing.T) {
	t.Parallel()

	r := require.New(t)

	edSgn := signing.NewEdSigner()

	tt := []struct {
		name    string
		message interface{}
		result  []byte
	}{
		{
			name:    "Case 1",
			message: []byte{},
			result:  edSgn.Sign([]byte{0, 0, 0, 0}),
		},
		{
			name:    "Case 2",
			message: &struct{ Test int }{Test: 0x12345678},
			result:  edSgn.Sign([]byte{0x12, 0x34, 0x56, 0x78}),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := signMessage(edSgn, tc.message, logtest.New(t))
			r.Equal(string(tc.result), string(result))
		})
	}
}

func TestTortoiseBeacon_getSignedProposal(t *testing.T) {
	t.Parallel()

	r := require.New(t)

	edSgn := signing.NewEdSigner()
	edPubkey := edSgn.PublicKey()

	vrfSigner, _, err := signing.NewVRFSigner(edSgn.Sign(edPubkey.Bytes()))
	r.NoError(err)

	tt := []struct {
		name   string
		epoch  types.EpochID
		result []byte
	}{
		{
			name:   "Case 1",
			epoch:  1,
			result: vrfSigner.Sign(util.Hex2Bytes("000000035442500000000001")),
		},
		{
			name:   "Case 2",
			epoch:  2,
			result: vrfSigner.Sign(util.Hex2Bytes("000000035442500000000002")),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := buildSignedProposal(context.TODO(), vrfSigner, tc.epoch, logtest.New(t))
			r.Equal(string(tc.result), string(result))
		})
	}
}

func TestTortoiseBeacon_signAndExtractED(t *testing.T) {
	r := require.New(t)

	signer := signing.NewEdSigner()
	verifier := signing.NewEDVerifier()

	message := []byte{1, 2, 3, 4}

	signature := signer.Sign(message)
	extractedPK, err := verifier.Extract(message, signature)
	r.NoError(err)

	ok := verifier.Verify(extractedPK, message, signature)

	r.Equal(signer.PublicKey().String(), extractedPK.String())
	r.True(ok)
}

func TestTortoiseBeacon_signAndVerifyVRF(t *testing.T) {
	r := require.New(t)

	signer, _, err := signing.NewVRFSigner(bytes.Repeat([]byte{0x01}, 32))
	r.NoError(err)

	verifier := signing.VRFVerifier{}

	message := []byte{1, 2, 3, 4}

	signature := signer.Sign(message)
	ok := verifier.Verify(signer.PublicKey(), message, signature)
	r.True(ok)
}
