package hare

import (
	"context"
	"errors"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p/service"
	"github.com/spacemeshos/go-spacemesh/priorityq"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"
)

type HareWrapper struct {
	totalCP     int
	termination Closer
	clock       *mockClock
	hare        []*Hare
	initialSets []*Set // all initial sets
	outputs     map[instanceID][]*Set
	name        string
}

func newHareWrapper(totalCp int) *HareWrapper {
	hs := new(HareWrapper)
	hs.clock = newMockClock()
	hs.totalCP = totalCp
	hs.termination = NewCloser()
	hs.outputs = make(map[instanceID][]*Set, 0)

	return hs
}

func (his *HareWrapper) fill(set *Set, begin int, end int) {
	for i := begin; i <= end; i++ {
		his.initialSets[i] = set
	}
}

func (his *HareWrapper) waitForTermination() {
	for {
		count := 0
		for _, p := range his.hare {
			for i := types.GetEffectiveGenesis() + 1; i <= types.GetEffectiveGenesis()+types.LayerID(his.totalCP); i++ {
				blks, _ := p.GetResult(i)
				if len(blks) > 0 {
					count++
				}
			}
		}

		// log.Info("count is %v", count)
		if count == his.totalCP*len(his.hare) {
			break
		}

		time.Sleep(300 * time.Millisecond)
	}

	log.Info("Terminating. Validating outputs")

	for _, p := range his.hare {
		for i := types.GetEffectiveGenesis() + 1; i <= types.GetEffectiveGenesis()+types.LayerID(his.totalCP); i++ {
			s := NewEmptySet(10)
			blks, _ := p.GetResult(i)
			for _, b := range blks {
				s.Add(b)
			}
			his.outputs[instanceID(i)] = append(his.outputs[instanceID(i)], s)
		}
	}

	his.termination.Close()
}

func (his *HareWrapper) WaitForTimedTermination(t *testing.T, timeout time.Duration) {
	timer := time.After(timeout)
	go his.waitForTermination()
	select {
	case <-timer:
		t.Fatal("Timeout")
		return
	case <-his.termination.CloseChannel():
		for i := 1; i <= his.totalCP; i++ {
			his.checkResult(t, instanceID(i))
		}
		return
	}
}

func (his *HareWrapper) checkResult(t *testing.T, id instanceID) {
	// check consistency
	out := his.outputs[id]
	for i := 0; i < len(out)-1; i++ {
		if !out[i].Equals(out[i+1]) {
			t.Errorf("Consistency check failed: Expected: %v Actual: %v", out[i], out[i+1])
		}
	}
}

type p2pManipulator struct {
	nd           *service.Node
	stalledLayer instanceID
	err          error
}

func (m *p2pManipulator) RegisterGossipProtocol(protocol string, prio priorityq.Priority) chan service.GossipMessage {
	ch := m.nd.RegisterGossipProtocol(protocol, prio)
	wch := make(chan service.GossipMessage)

	go func() {
		for {
			x := <-ch
			wch <- x
		}
	}()

	return wch
}

func (m *p2pManipulator) Broadcast(ctx context.Context, protocol string, payload []byte) error {
	msg, _ := MessageFromBuffer(payload)
	if msg.InnerMsg.InstanceID == m.stalledLayer && msg.InnerMsg.K < 8 && msg.InnerMsg.K != -1 {
		log.Warning("Not broadcasting in manipulator")
		return m.err
	}

	e := m.nd.Broadcast(ctx, protocol, payload)
	return e
}

type trueOracle struct {
}

func (trueOracle) Register(bool, string) {
}

func (trueOracle) Unregister(bool, string) {
}

func (trueOracle) Validate(context.Context, types.LayerID, int32, int, types.NodeID, []byte, uint16) (bool, error) {
	return true, nil
}

func (trueOracle) CalcEligibility(context.Context, types.LayerID, int32, int, types.NodeID, []byte) (uint16, error) {
	return 1, nil
}

func (trueOracle) Proof(context.Context, types.LayerID, int32) ([]byte, error) {
	x := make([]byte, 100)
	return x, nil
}

func (trueOracle) IsIdentityActiveOnConsensusView(string, types.LayerID) (bool, error) {
	return true, nil
}

// Test - runs a single CP for more than one iteration
func Test_consensusIterations(t *testing.T) {
	test := newConsensusTest()

	totalNodes := 20
	cfg := config.Config{N: totalNodes, F: totalNodes/2 - 1, RoundDuration: 1, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100}
	sim := service.NewSimulator()
	test.initialSets = make([]*Set, totalNodes)
	set1 := NewSetFromValues(value1)
	test.fill(set1, 0, totalNodes-1)
	test.honestSets = []*Set{set1}
	oracle := &trueOracle{}
	i := 0
	creationFunc := func() {
		s := sim.NewNode()
		p2pm := &p2pManipulator{nd: s, stalledLayer: 1, err: errors.New("fake err")}
		proc := createConsensusProcess(true, cfg, oracle, p2pm, test.initialSets[i], 1, t.Name())
		test.procs = append(test.procs, proc)
		i++
	}
	test.Create(totalNodes, creationFunc)
	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}

func validateBlock([]types.BlockID) bool {
	return true
}

func isSynced(context.Context) bool {
	return true
}

type mockIdentityP struct {
	nid types.NodeID
}

func (m *mockIdentityP) GetIdentity(string) (types.NodeID, error) {
	return m.nid, nil
}

func buildSet() []types.BlockID {
	rng := rand.New(rand.NewSource(0))
	s := make([]types.BlockID, 200, 200)
	for i := uint64(0); i < 200; i++ {
		s = append(s, newRandBlockID(rng))
	}
	return s
}

func newRandBlockID(rng *rand.Rand) (id types.BlockID) {
	_, err := rng.Read(id[:])
	if err != nil {
		panic(err)
	}
	return id
}

type mockBlockProvider struct {
}

func (mbp *mockBlockProvider) HandleValidatedLayer(context.Context, types.LayerID, []types.BlockID) {
}

func (mbp *mockBlockProvider) LayerBlockIds(types.LayerID) ([]types.BlockID, error) {
	return buildSet(), nil
}

func createMaatuf(tcfg config.Config, clock *mockClock, p2p NetworkService, rolacle Rolacle, name string) *Hare {
	ed := signing.NewEdSigner()
	pub := ed.PublicKey()
	_, vrfPub, err := signing.NewVRFSigner(ed.Sign(pub.Bytes()))
	if err != nil {
		panic("failed to create vrf signer")
	}
	nodeID := types.NodeID{Key: pub.String(), VRFPublicKey: vrfPub}
	hare := New(tcfg, p2p, ed, nodeID, validateBlock, isSynced, &mockBlockProvider{}, rolacle, 10, &mockIdentityP{nid: nodeID},
		&MockStateQuerier{true, nil}, clock, log.NewDefault(name+"_"+ed.PublicKey().ShortString()))

	return hare
}

type mockClock struct {
	channels     map[types.LayerID]chan struct{}
	layerTime    map[types.LayerID]time.Time
	currentLayer types.LayerID
	m            sync.RWMutex
}

func newMockClock() *mockClock {
	return &mockClock{
		channels:     make(map[types.LayerID]chan struct{}),
		layerTime:    map[types.LayerID]time.Time{types.GetEffectiveGenesis(): time.Now()},
		currentLayer: types.GetEffectiveGenesis() + 1,
	}
}

func (m *mockClock) LayerToTime(layer types.LayerID) time.Time {
	m.m.RLock()
	defer m.m.RUnlock()

	return m.layerTime[layer]
}

func (m *mockClock) AwaitLayer(layer types.LayerID) chan struct{} {
	m.m.Lock()
	defer m.m.Unlock()

	if _, ok := m.layerTime[layer]; ok {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if ch, ok := m.channels[layer]; ok {
		return ch
	}
	ch := make(chan struct{})
	m.channels[layer] = ch
	return ch
}

func (m *mockClock) GetCurrentLayer() types.LayerID {
	m.m.RLock()
	defer m.m.RUnlock()

	return m.currentLayer
}

func (m *mockClock) advanceLayer() {
	time.Sleep(time.Millisecond)
	m.m.Lock()
	defer m.m.Unlock()

	log.Info("sending for layer %v", m.currentLayer)
	m.layerTime[m.currentLayer] = time.Now()
	if ch, ok := m.channels[m.currentLayer]; ok {
		close(ch)
		delete(m.channels, m.currentLayer)
	}
	m.currentLayer++
}

type SharedRoundClock struct {
	currentRound     int32
	rounds           map[int32]chan struct{}
	minCount         int
	processingDelay  time.Duration
	sentMessages     uint16
	advanceScheduled bool
	m                sync.Mutex
}

func newSharedClock(minCount int, totalCP int, processingDelay time.Duration) map[instanceID]*SharedRoundClock {
	m := make(map[instanceID]*SharedRoundClock)
	for i := types.GetEffectiveGenesis() + 1; i <= types.GetEffectiveGenesis()+types.LayerID(totalCP); i++ {
		m[instanceID(i)] = &SharedRoundClock{
			currentRound:    -1,
			rounds:          make(map[int32]chan struct{}),
			minCount:        minCount,
			processingDelay: processingDelay,
			sentMessages:    0,
		}
	}
	return m
}

func (c *SharedRoundClock) AwaitWakeup() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (c *SharedRoundClock) AwaitEndOfRound(round int32) <-chan struct{} {
	c.m.Lock()
	defer c.m.Unlock()

	ch, ok := c.rounds[round]
	if !ok {
		ch = make(chan struct{})
		c.rounds[round] = ch
	}
	return ch
}

func (c *SharedRoundClock) IncMessages(cnt uint16) {
	c.m.Lock()
	defer c.m.Unlock()

	c.sentMessages += cnt
	if int(c.sentMessages) >= c.minCount && !c.advanceScheduled {
		time.AfterFunc(c.processingDelay, func() {
			c.m.Lock()
			defer c.m.Unlock()

			c.sentMessages = 0
			c.advanceScheduled = false
			c.advanceRound()
		})
		c.advanceScheduled = true
	}
}

func (c *SharedRoundClock) advanceRound() {
	c.currentRound++
	if prevRound, ok := c.rounds[c.currentRound-1]; ok {
		close(prevRound)
	}
}

type SimRoundClock struct {
	clocks map[instanceID]*SharedRoundClock
	m      sync.Mutex
	s      NetworkService
}

func (c *SimRoundClock) RegisterGossipProtocol(protocol string, prio priorityq.Priority) chan service.GossipMessage {
	return c.s.RegisterGossipProtocol(protocol, prio)
}

func (c *SimRoundClock) Broadcast(ctx context.Context, protocol string, payload []byte) error {
	instanceID, cnt := extractInstanceID(payload)

	c.m.Lock()
	clock := c.clocks[instanceID]
	clock.IncMessages(cnt)
	c.m.Unlock()

	return c.s.Broadcast(ctx, protocol, payload)
}

func extractInstanceID(payload []byte) (instanceID, uint16) {
	m, err := MessageFromBuffer(payload)
	if err != nil {
		panic(err)
	}
	return m.InnerMsg.InstanceID, m.InnerMsg.EligibilityCount
}

func (c *SimRoundClock) NewRoundClock(layerID types.LayerID) RoundClock {
	return c.clocks[instanceID(layerID)]
}

func NewSimRoundClock(s NetworkService, clocks map[instanceID]*SharedRoundClock) *SimRoundClock {
	return &SimRoundClock{
		clocks: clocks,
		s:      s,
	}
}

// Test - run multiple CPs simultaneously
func Test_multipleCPs(t *testing.T) {
	types.SetLayersPerEpoch(4)
	r := require.New(t)
	totalCp := 3
	test := newHareWrapper(totalCp)
	totalNodes := 20
	cfg := config.Config{N: totalNodes, F: totalNodes/2 - 1, RoundDuration: 5, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100}
	sim := service.NewSimulator()
	test.initialSets = make([]*Set, totalNodes)
	oracle := &trueOracle{}
	scMap := newSharedClock(totalNodes, totalCp, time.Duration(50*totalCp*totalNodes)*time.Millisecond)
	for i := 0; i < totalNodes; i++ {
		s := sim.NewNode()
		src := NewSimRoundClock(s, scMap)
		h := createMaatuf(cfg, test.clock, src, oracle, t.Name())
		h.newRoundClock = src.NewRoundClock
		test.hare = append(test.hare, h)
		e := h.Start(context.TODO())
		r.NoError(e)
	}

	go func() {
		for j := types.GetEffectiveGenesis() + 1; j <= types.GetEffectiveGenesis()+types.LayerID(totalCp); j++ {
			test.clock.advanceLayer()
			time.Sleep(250 * time.Millisecond)
		}
	}()

	test.WaitForTimedTermination(t, 30*time.Second)
}

// Test - run multiple CPs where one of them runs more than one iteration
func Test_multipleCPsAndIterations(t *testing.T) {
	types.SetLayersPerEpoch(4)
	r := require.New(t)
	totalCp := 4
	test := newHareWrapper(totalCp)
	totalNodes := 20
	cfg := config.Config{N: totalNodes, F: totalNodes/2 - 1, RoundDuration: 6, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100}
	sim := service.NewSimulator()
	test.initialSets = make([]*Set, totalNodes)
	oracle := &trueOracle{}
	scMap := newSharedClock(totalNodes, totalCp, time.Duration(50*totalCp*totalNodes)*time.Millisecond)
	for i := 0; i < totalNodes; i++ {
		s := sim.NewNode()
		mp2p := &p2pManipulator{nd: s, stalledLayer: 1, err: errors.New("fake err")}
		src := NewSimRoundClock(mp2p, scMap)
		h := createMaatuf(cfg, test.clock, src, oracle, t.Name())
		h.newRoundClock = src.NewRoundClock
		test.hare = append(test.hare, h)
		e := h.Start(context.TODO())
		r.NoError(e)
	}

	go func() {
		for j := types.GetEffectiveGenesis() + 1; j <= types.GetEffectiveGenesis()+types.LayerID(totalCp); j++ {
			test.clock.advanceLayer()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	test.WaitForTimedTermination(t, 40*time.Second)
}
