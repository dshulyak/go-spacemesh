package hare

import (
	"context"
	"errors"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/eligibility"
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p/node"
	"github.com/spacemeshos/go-spacemesh/p2p/service"
	"github.com/spacemeshos/go-spacemesh/priorityq"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var cfg = config.Config{N: 10, F: 5, RoundDuration: 2, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 1000}

type mockMessageValidator struct {
	syntaxValid  bool
	contextValid error
	countSyntax  int
	countContext int
}

func (mmv *mockMessageValidator) SyntacticallyValidateMessage(context.Context, *Msg) bool {
	mmv.countSyntax++
	return mmv.syntaxValid
}

func (mmv *mockMessageValidator) ContextuallyValidateMessage(context.Context, *Msg, int32) error {
	mmv.countContext++
	return mmv.contextValid
}

type mockRolacle struct {
	isEligible bool
	err        error
	MockStateQuerier
}

func (mr *mockRolacle) Validate(context.Context, types.LayerID, int32, int, types.NodeID, []byte, uint16) (bool, error) {
	return mr.isEligible, mr.err
}

func (mr *mockRolacle) CalcEligibility(context.Context, types.LayerID, int32, int, types.NodeID, []byte) (uint16, error) {
	if mr.isEligible {
		return 1, nil
	}
	return 0, mr.err
}

func (mr *mockRolacle) Proof(context.Context, types.LayerID, int32) ([]byte, error) {
	return []byte{}, nil
}

type mockP2p struct {
	count int
	err   error
}

func (m *mockP2p) RegisterGossipProtocol(string, priorityq.Priority) chan service.GossipMessage {
	return make(chan service.GossipMessage)
}

func (m *mockP2p) Broadcast(context.Context, string, []byte) error {
	m.count++
	return m.err
}

type mockProposalTracker struct {
	isConflicting       bool
	proposedSet         *Set
	countOnProposal     int
	countOnLateProposal int
	countIsConflicting  int
	countProposedSet    int
}

func (mpt *mockProposalTracker) OnProposal(context.Context, *Msg) {
	mpt.countOnProposal++
}

func (mpt *mockProposalTracker) OnLateProposal(context.Context, *Msg) {
	mpt.countOnLateProposal++
}

func (mpt *mockProposalTracker) IsConflicting() bool {
	mpt.countIsConflicting++
	return mpt.isConflicting
}

func (mpt *mockProposalTracker) ProposedSet() *Set {
	mpt.countProposedSet++
	return mpt.proposedSet
}

type mockCommitTracker struct {
	countOnCommit         int
	countHasEnoughCommits int
	countBuildCertificate int
	hasEnoughCommits      bool
	certificate           *certificate
}

func (mct *mockCommitTracker) CommitCount() int {
	return 0
}

func (mct *mockCommitTracker) OnCommit(*Msg) {
	mct.countOnCommit++
}

func (mct *mockCommitTracker) HasEnoughCommits() bool {
	mct.countHasEnoughCommits++
	return mct.hasEnoughCommits
}

func (mct *mockCommitTracker) BuildCertificate() *certificate {
	mct.countBuildCertificate++
	return mct.certificate
}

type mockSyncer struct {
	isSync bool
}

func (s *mockSyncer) IsSynced(context.Context) bool {
	return s.isSync
}

func generateSigning() Signer {
	return signing.NewEdSigner()
}

func buildMessage(msg *Message) *Msg {
	return &Msg{Message: msg, PubKey: nil}
}

func buildBroker(net NetworkService, testName string) *Broker {
	return newBroker(net, &mockEligibilityValidator{valid: true}, MockStateQuerier{true, nil},
		(&mockSyncer{true}).IsSynced, 10, cfg.LimitIterations, Closer{make(chan struct{})}, log.NewDefault(testName))
}

func buildBrokerLimit4(net NetworkService, testName string) *Broker {
	return newBroker(net, &mockEligibilityValidator{valid: true}, MockStateQuerier{true, nil},
		(&mockSyncer{true}).IsSynced, 10, 4, Closer{make(chan struct{})}, log.NewDefault(testName))
}

type mockEligibilityValidator struct {
	valid        bool
	validationFn func(context.Context, *Msg) bool
}

func (mev *mockEligibilityValidator) Validate(ctx context.Context, msg *Msg) bool {
	if mev.validationFn != nil {
		return mev.validationFn(ctx, msg)
	}
	return mev.valid
}

// test that a InnerMsg to a specific set ObjectID is delivered by the broker
func TestConsensusProcess_Start(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	broker := buildBroker(n1, t.Name())
	err := broker.Start(context.TODO())
	assert.NoError(t, err)
	proc := generateConsensusProcess(t, cfg)
	inbox, _ := broker.Register(context.TODO(), proc.ID())
	proc.SetInbox(inbox)
	proc.s = NewSetFromValues(value1)
	err = proc.Start(context.TODO())
	assert.NoError(t, err)
	err = proc.Start(context.TODO())
	assert.EqualError(t, err, "instance already started")
}

func TestConsensusProcess_TerminationLimit(t *testing.T) {
	myCfg := cfg
	myCfg.LimitIterations = 1
	myCfg.RoundDuration = 1
	p := generateConsensusProcess(t, myCfg)
	p.SetInbox(make(chan *Msg, 10))
	err := p.Start(context.TODO())
	assert.NoError(t, err)
	time.Sleep(time.Duration(6*p.cfg.RoundDuration) * time.Second)
	assert.Equal(t, int32(1), p.k/4)
}

func TestConsensusProcess_eventLoop(t *testing.T) {
	net := &mockP2p{}
	broker := buildBroker(net, t.Name())
	err := broker.Start(context.TODO())
	assert.NoError(t, err)
	proc := generateConsensusProcess(t, cfg)
	proc.network = net
	oracle := &mockRolacle{MockStateQuerier: MockStateQuerier{true, nil}}
	oracle.isEligible = true
	proc.oracle = oracle
	proc.inbox, _ = broker.Register(context.TODO(), proc.ID())
	proc.s = NewSetFromValues(value1, value2)
	proc.cfg.F = 2
	go proc.eventLoop(context.TODO())
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 1, net.count)
}

func TestConsensusProcess_handleMessage(t *testing.T) {
	r := require.New(t)
	net := &mockP2p{}
	broker := buildBroker(net, t.Name())
	err := broker.Start(context.TODO())
	assert.NoError(t, err)
	proc := generateConsensusProcess(t, cfg)
	proc.network = net
	oracle := &mockRolacle{}
	proc.oracle = oracle
	mValidator := &mockMessageValidator{}
	proc.validator = mValidator
	proc.inbox, _ = broker.Register(context.TODO(), proc.ID())
	msg := BuildPreRoundMsg(generateSigning(), NewSetFromValues(value1))
	oracle.isEligible = true
	mValidator.syntaxValid = false
	proc.handleMessage(context.TODO(), msg)
	r.Equal(1, mValidator.countSyntax)
	r.Equal(1, mValidator.countContext)
	mValidator.syntaxValid = true
	proc.handleMessage(context.TODO(), msg)
	r.NotEqual(0, mValidator.countContext)
	mValidator.contextValid = nil
	proc.handleMessage(context.TODO(), msg)
	r.Equal(0, len(proc.pending))
	r.Equal(3, mValidator.countContext)
	r.Equal(3, mValidator.countSyntax)
	mValidator.contextValid = errors.New("not valid")
	proc.handleMessage(context.TODO(), msg)
	r.Equal(4, mValidator.countContext)
	r.Equal(3, mValidator.countSyntax)
	r.Equal(0, len(proc.pending))
	mValidator.contextValid = errEarlyMsg
	proc.handleMessage(context.TODO(), msg)
	r.Equal(1, len(proc.pending))
}

func TestConsensusProcess_nextRound(t *testing.T) {
	sim := service.NewSimulator()
	n1 := sim.NewNode()
	broker := buildBroker(n1, t.Name())
	err := broker.Start(context.TODO())
	assert.NoError(t, err)
	proc := generateConsensusProcess(t, cfg)
	proc.inbox, _ = broker.Register(context.TODO(), proc.ID())
	proc.advanceToNextRound(context.TODO())
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, int32(1), proc.k)
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, int32(2), proc.k)
}

func generateConsensusProcess(t *testing.T, cfg config.Config) *consensusProcess {
	_, bninfo := node.GenerateTestNode(t)
	sim := service.NewSimulator()
	n1 := sim.NewNodeFrom(bninfo)

	s := NewSetFromValues(value1)
	oracle := eligibility.New()
	edSigner := signing.NewEdSigner()
	edPubkey := edSigner.PublicKey()
	_, vrfPub, err := signing.NewVRFSigner(edSigner.Sign(edPubkey.Bytes()))
	assert.NoError(t, err)
	oracle.Register(true, edPubkey.String())
	output := make(chan TerminationOutput, 1)
	return newConsensusProcess(cfg, instanceID1, s, oracle, NewMockStateQuerier(), 10, edSigner,
		types.NodeID{Key: edPubkey.String(), VRFPublicKey: vrfPub}, n1, output, truer{}, newRoundClockFromCfg(cfg),
		log.NewDefault(edPubkey.String()))
}

func TestConsensusProcess_Id(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.instanceID = instanceID1
	assert.Equal(t, instanceID1, proc.ID())
}

func TestNewConsensusProcess_AdvanceToNextRound(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	k := proc.k
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, k+1, proc.k)
}

func TestConsensusProcess_CreateInbox(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	inbox := make(chan *Msg, 100)
	proc.SetInbox(inbox)
	assert.NotNil(t, proc.inbox)
	assert.Equal(t, 100, cap(proc.inbox))
}

func TestConsensusProcess_InitDefaultBuilder(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	s := NewEmptySet(defaultSetSize)
	s.Add(value1)
	builder, err := proc.initDefaultBuilder(s)
	assert.Nil(t, err)
	assert.True(t, NewSet(builder.inner.Values).Equals(s))
	verifier := builder.msg.PubKey
	assert.Nil(t, verifier)
	assert.Equal(t, builder.inner.K, proc.k)
	assert.Equal(t, builder.inner.Ki, proc.ki)
	assert.Equal(t, builder.inner.InstanceID, proc.instanceID)
}

func TestConsensusProcess_isEligible(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	oracle := &mockRolacle{MockStateQuerier: MockStateQuerier{true, nil}}
	proc.oracle = oracle
	oracle.isEligible = false
	assert.False(t, proc.shouldParticipate(context.TODO()))
	oracle.isEligible = true
	assert.True(t, proc.shouldParticipate(context.TODO()))
	oracle.MockStateQuerier = MockStateQuerier{false, errors.New("some err")}
	assert.False(t, proc.shouldParticipate(context.TODO()))
	oracle.MockStateQuerier = MockStateQuerier{false, nil}
	assert.False(t, proc.shouldParticipate(context.TODO()))
	oracle.MockStateQuerier = MockStateQuerier{true, nil}
	assert.True(t, proc.shouldParticipate(context.TODO()))
}

func TestConsensusProcess_sendMessage(t *testing.T) {
	r := require.New(t)
	net := &mockP2p{}

	proc := generateConsensusProcess(t, cfg)
	proc.network = net

	b := proc.sendMessage(context.TODO(), nil)
	r.Equal(0, net.count)
	r.False(b)
	msg := buildStatusMsg(generateSigning(), proc.s, 0)

	net.err = errors.New("mock network failed error")
	b = proc.sendMessage(context.TODO(), msg)
	r.False(b)
	r.Equal(1, net.count)

	net.err = nil
	b = proc.sendMessage(context.TODO(), msg)
	r.True(b)
}

func TestConsensusProcess_procPre(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	s := NewDefaultEmptySet()
	m := BuildPreRoundMsg(generateSigning(), s)
	proc.processPreRoundMsg(context.TODO(), m)
	assert.Equal(t, 1, len(proc.preRoundTracker.preRound))
}

func TestConsensusProcess_procStatus(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.beginStatusRound(context.TODO())
	s := NewDefaultEmptySet()
	m := BuildStatusMsg(generateSigning(), s)
	proc.processStatusMsg(context.TODO(), m)
	assert.Equal(t, 1, len(proc.statusesTracker.statuses))
}

func TestConsensusProcess_procProposal(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.validator.(*syntaxContextValidator).threshold = 1
	proc.validator.(*syntaxContextValidator).statusValidator = func(m *Msg) bool {
		return true
	}
	proc.SetInbox(make(chan *Msg))
	proc.advanceToNextRound(context.TODO())
	proc.advanceToNextRound(context.TODO())
	s := NewSetFromValues(value1)
	m := BuildProposalMsg(generateSigning(), s)
	mpt := &mockProposalTracker{}
	proc.proposalTracker = mpt
	proc.handleMessage(context.TODO(), m)
	assert.Equal(t, 1, mpt.countOnProposal)
	proc.advanceToNextRound(context.TODO())
	proc.handleMessage(context.TODO(), m)
	assert.Equal(t, 1, mpt.countOnLateProposal)
}

func TestConsensusProcess_procCommit(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	s := NewDefaultEmptySet()
	proc.commitTracker = newCommitTracker(1, 1, s)
	m := BuildCommitMsg(generateSigning(), s)
	mct := &mockCommitTracker{}
	proc.commitTracker = mct
	proc.processCommitMsg(context.TODO(), m)
	assert.Equal(t, 1, mct.countOnCommit)
}

func TestConsensusProcess_procNotify(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	s := NewSetFromValues(value1)
	m := BuildNotifyMsg(generateSigning(), s)
	proc.processNotifyMsg(context.TODO(), m)
	assert.Equal(t, 1, len(proc.notifyTracker.notifies))
	m = BuildNotifyMsg(generateSigning(), s)
	proc.ki = 0
	m.InnerMsg.K = proc.ki
	proc.s.Add(value5)
	proc.k = notifyRound
	proc.processNotifyMsg(context.TODO(), m)
	assert.True(t, s.Equals(proc.s))
}

func TestConsensusProcess_Termination(t *testing.T) {

	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	s := NewSetFromValues(value1)

	for i := 0; i < cfg.F+1; i++ {
		proc.processNotifyMsg(context.TODO(), BuildNotifyMsg(generateSigning(), s))
	}

	timer := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-timer.C:
			t.Fatal("Timeout")
		case <-proc.CloseChannel():
			return
		}
	}
}

func TestConsensusProcess_currentRound(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, statusRound, proc.currentRound())
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, proposalRound, proc.currentRound())
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, commitRound, proc.currentRound())
	proc.advanceToNextRound(context.TODO())
	assert.Equal(t, notifyRound, proc.currentRound())
}

func TestConsensusProcess_onEarlyMessage(t *testing.T) {
	r := require.New(t)
	proc := generateConsensusProcess(t, cfg)
	m := BuildPreRoundMsg(generateSigning(), NewDefaultEmptySet())
	proc.advanceToNextRound(context.TODO())
	proc.onEarlyMessage(context.TODO(), buildMessage(nil))
	r.Equal(0, len(proc.pending))
	proc.onEarlyMessage(context.TODO(), m)
	r.Equal(1, len(proc.pending))
	proc.onEarlyMessage(context.TODO(), m)
	r.Equal(1, len(proc.pending))
	m2 := BuildPreRoundMsg(generateSigning(), NewDefaultEmptySet())
	proc.onEarlyMessage(context.TODO(), m2)
	m3 := BuildPreRoundMsg(generateSigning(), NewDefaultEmptySet())
	proc.onEarlyMessage(context.TODO(), m3)
	r.Equal(3, len(proc.pending))
	proc.onRoundBegin(context.TODO())

	// make sure we wait enough for the go routine to be executed
	timeout := time.NewTimer(1 * time.Second)
	tk := time.NewTicker(100 * time.Microsecond)
	select {
	case <-timeout.C:
		r.Equal(0, len(proc.pending))
	case <-tk.C:
		if 0 == len(proc.pending) {
			return
		}
	}
}

func TestProcOutput_Id(t *testing.T) {
	po := procReport{instanceID1, nil, false}
	assert.Equal(t, po.ID(), instanceID1)
}

func TestProcOutput_Set(t *testing.T) {
	es := NewDefaultEmptySet()
	po := procReport{instanceID1, es, false}
	assert.True(t, es.Equals(po.Set()))
}

func TestIterationFromCounter(t *testing.T) {
	for i := 0; i < 10; i++ {
		assert.Equal(t, int32(i/4), iterationFromCounter(int32(i)))
	}
}

func TestConsensusProcess_beginRound1(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	proc.onRoundBegin(context.TODO())
	network := &mockP2p{}
	proc.network = network
	oracle := &mockRolacle{MockStateQuerier: MockStateQuerier{true, nil}}
	proc.oracle = oracle
	s := NewDefaultEmptySet()
	m := BuildPreRoundMsg(generateSigning(), s)
	proc.statusesTracker.RecordStatus(context.TODO(), m)

	preStatusTracker := proc.statusesTracker
	oracle.isEligible = true
	proc.beginStatusRound(context.TODO())
	assert.Equal(t, 1, network.count)
	assert.NotEqual(t, preStatusTracker, proc.statusesTracker)
}

func TestConsensusProcess_beginRound2(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.advanceToNextRound(context.TODO())
	network := &mockP2p{}
	proc.network = network
	oracle := &mockRolacle{MockStateQuerier: MockStateQuerier{true, nil}}
	proc.oracle = oracle
	oracle.isEligible = true

	statusTracker := newStatusTracker(1, 1)
	s := NewSetFromValues(value1)
	statusTracker.RecordStatus(context.TODO(), BuildStatusMsg(generateSigning(), s))
	statusTracker.AnalyzeStatuses(validate)
	proc.statusesTracker = statusTracker

	proc.k = 1
	proc.SetInbox(make(chan *Msg, 1))
	proc.beginProposalRound(context.TODO())

	assert.Equal(t, 1, network.count)
	assert.Nil(t, proc.statusesTracker)
}

func TestConsensusProcess_beginRound3(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	network := &mockP2p{}
	proc.network = network
	oracle := &mockRolacle{MockStateQuerier: MockStateQuerier{true, nil}}
	proc.oracle = oracle
	mpt := &mockProposalTracker{}
	proc.proposalTracker = mpt
	mpt.proposedSet = NewSetFromValues(value1)

	preCommitTracker := proc.commitTracker
	proc.beginCommitRound(context.TODO())
	assert.NotEqual(t, preCommitTracker, proc.commitTracker)

	mpt.isConflicting = false
	mpt.proposedSet = NewSetFromValues(value1)
	oracle.isEligible = true
	proc.SetInbox(make(chan *Msg, 1))
	proc.beginCommitRound(context.TODO())
	assert.Equal(t, 1, network.count)
}

type mockNet struct {
	callBroadcast int
	callRegister  int
	err           error
}

func (m *mockNet) Broadcast(context.Context, string, []byte) error {
	m.callBroadcast++
	return m.err
}

func (m *mockNet) RegisterGossipProtocol(string, priorityq.Priority) chan service.GossipMessage {
	m.callRegister++
	return nil
}

func TestConsensusProcess_handlePending(t *testing.T) {
	proc := generateConsensusProcess(t, cfg)
	proc.SetInbox(make(chan *Msg, 100))
	const count = 5
	pending := make(map[string]*Msg)
	for i := 0; i < count; i++ {
		v := generateSigning()
		m := BuildStatusMsg(v, NewSetFromValues(value1))
		pending[v.PublicKey().String()] = m
	}
	proc.handlePending(pending)
	assert.Equal(t, count, len(proc.inbox))
}

func TestConsensusProcess_beginRound4(t *testing.T) {
	r := require.New(t)

	proc := generateConsensusProcess(t, cfg)
	net := &mockNet{}
	proc.network = net
	mpt := &mockProposalTracker{}
	mct := &mockCommitTracker{}
	proc.proposalTracker = mpt
	proc.commitTracker = mct
	proc.beginNotifyRound(context.TODO())
	r.Equal(1, mpt.countIsConflicting)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)

	proc.proposalTracker = mpt
	proc.commitTracker = mct
	mpt.isConflicting = true
	proc.beginNotifyRound(context.TODO())
	r.Equal(2, mpt.countIsConflicting)
	r.Equal(1, mct.countHasEnoughCommits)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)

	proc.proposalTracker = mpt
	proc.commitTracker = mct
	mpt.isConflicting = false
	mct.hasEnoughCommits = false
	proc.beginNotifyRound(context.TODO())
	r.Equal(0, mct.countBuildCertificate)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)

	proc.proposalTracker = mpt
	proc.commitTracker = mct
	mct.hasEnoughCommits = true
	mct.certificate = nil
	proc.beginNotifyRound(context.TODO())
	r.Equal(1, mct.countBuildCertificate)
	r.Equal(0, mpt.countProposedSet)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)

	proc.proposalTracker = mpt
	proc.commitTracker = mct
	mct.certificate = &certificate{}
	mpt.proposedSet = nil
	proc.s = NewDefaultEmptySet()
	proc.beginNotifyRound(context.TODO())
	r.NotNil(proc.s)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)

	proc.proposalTracker = mpt
	proc.commitTracker = mct
	mpt.proposedSet = NewSetFromValues(value1)
	proc.beginNotifyRound(context.TODO())
	r.True(proc.s.Equals(mpt.proposedSet))
	r.Equal(mct.certificate, proc.certificate)
	r.Nil(proc.proposalTracker)
	r.Nil(proc.commitTracker)
	r.Equal(1, net.callBroadcast)
	r.True(proc.notifySent)
}
