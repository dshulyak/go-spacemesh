// Code generated by MockGen. DO NOT EDIT.
// Source: ./interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/spacemeshos/go-spacemesh/common/types"
	signing "github.com/spacemeshos/go-spacemesh/signing"
)

// MockatxDB is a mock of atxDB interface.
type MockatxDB struct {
	ctrl     *gomock.Controller
	recorder *MockatxDBMockRecorder
}

// MockatxDBMockRecorder is the mock recorder for MockatxDB.
type MockatxDBMockRecorder struct {
	mock *MockatxDB
}

// NewMockatxDB creates a new mock instance.
func NewMockatxDB(ctrl *gomock.Controller) *MockatxDB {
	mock := &MockatxDB{ctrl: ctrl}
	mock.recorder = &MockatxDBMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockatxDB) EXPECT() *MockatxDBMockRecorder {
	return m.recorder
}

// GetAtxHeader mocks base method.
func (m *MockatxDB) GetAtxHeader(arg0 types.ATXID) (*types.ActivationTxHeader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAtxHeader", arg0)
	ret0, _ := ret[0].(*types.ActivationTxHeader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAtxHeader indicates an expected call of GetAtxHeader.
func (mr *MockatxDBMockRecorder) GetAtxHeader(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAtxHeader", reflect.TypeOf((*MockatxDB)(nil).GetAtxHeader), arg0)
}

// MockballotDB is a mock of ballotDB interface.
type MockballotDB struct {
	ctrl     *gomock.Controller
	recorder *MockballotDBMockRecorder
}

// MockballotDBMockRecorder is the mock recorder for MockballotDB.
type MockballotDBMockRecorder struct {
	mock *MockballotDB
}

// NewMockballotDB creates a new mock instance.
func NewMockballotDB(ctrl *gomock.Controller) *MockballotDB {
	mock := &MockballotDB{ctrl: ctrl}
	mock.recorder = &MockballotDBMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockballotDB) EXPECT() *MockballotDBMockRecorder {
	return m.recorder
}

// GetBallot mocks base method.
func (m *MockballotDB) GetBallot(arg0 types.BallotID) (*types.Ballot, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBallot", arg0)
	ret0, _ := ret[0].(*types.Ballot)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBallot indicates an expected call of GetBallot.
func (mr *MockballotDBMockRecorder) GetBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBallot", reflect.TypeOf((*MockballotDB)(nil).GetBallot), arg0)
}

// MockmeshDB is a mock of meshDB interface.
type MockmeshDB struct {
	ctrl     *gomock.Controller
	recorder *MockmeshDBMockRecorder
}

// MockmeshDBMockRecorder is the mock recorder for MockmeshDB.
type MockmeshDBMockRecorder struct {
	mock *MockmeshDB
}

// NewMockmeshDB creates a new mock instance.
func NewMockmeshDB(ctrl *gomock.Controller) *MockmeshDB {
	mock := &MockmeshDB{ctrl: ctrl}
	mock.recorder = &MockmeshDBMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockmeshDB) EXPECT() *MockmeshDBMockRecorder {
	return m.recorder
}

// AddBallot mocks base method.
func (m *MockmeshDB) AddBallot(arg0 *types.Ballot) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddBallot", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBallot indicates an expected call of AddBallot.
func (mr *MockmeshDBMockRecorder) AddBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddBallot", reflect.TypeOf((*MockmeshDB)(nil).AddBallot), arg0)
}

// AddTXsFromProposal mocks base method.
func (m *MockmeshDB) AddTXsFromProposal(arg0 context.Context, arg1 types.LayerID, arg2 types.ProposalID, arg3 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTXsFromProposal", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddTXsFromProposal indicates an expected call of AddTXsFromProposal.
func (mr *MockmeshDBMockRecorder) AddTXsFromProposal(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTXsFromProposal", reflect.TypeOf((*MockmeshDB)(nil).AddTXsFromProposal), arg0, arg1, arg2, arg3)
}

// GetBallot mocks base method.
func (m *MockmeshDB) GetBallot(arg0 types.BallotID) (*types.Ballot, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBallot", arg0)
	ret0, _ := ret[0].(*types.Ballot)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBallot indicates an expected call of GetBallot.
func (mr *MockmeshDBMockRecorder) GetBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBallot", reflect.TypeOf((*MockmeshDB)(nil).GetBallot), arg0)
}

// GetBlockLayer mocks base method.
func (m *MockmeshDB) GetBlockLayer(arg0 types.BlockID) (types.LayerID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockLayer", arg0)
	ret0, _ := ret[0].(types.LayerID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockLayer indicates an expected call of GetBlockLayer.
func (mr *MockmeshDBMockRecorder) GetBlockLayer(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockLayer", reflect.TypeOf((*MockmeshDB)(nil).GetBlockLayer), arg0)
}

// HasBallot mocks base method.
func (m *MockmeshDB) HasBallot(arg0 types.BallotID) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasBallot", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasBallot indicates an expected call of HasBallot.
func (mr *MockmeshDBMockRecorder) HasBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasBallot", reflect.TypeOf((*MockmeshDB)(nil).HasBallot), arg0)
}

// SetIdentityMalicious mocks base method.
func (m *MockmeshDB) SetIdentityMalicious(arg0 *signing.PublicKey) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetIdentityMalicious", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetIdentityMalicious indicates an expected call of SetIdentityMalicious.
func (mr *MockmeshDBMockRecorder) SetIdentityMalicious(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetIdentityMalicious", reflect.TypeOf((*MockmeshDB)(nil).SetIdentityMalicious), arg0)
}

// MockproposalDB is a mock of proposalDB interface.
type MockproposalDB struct {
	ctrl     *gomock.Controller
	recorder *MockproposalDBMockRecorder
}

// MockproposalDBMockRecorder is the mock recorder for MockproposalDB.
type MockproposalDBMockRecorder struct {
	mock *MockproposalDB
}

// NewMockproposalDB creates a new mock instance.
func NewMockproposalDB(ctrl *gomock.Controller) *MockproposalDB {
	mock := &MockproposalDB{ctrl: ctrl}
	mock.recorder = &MockproposalDBMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockproposalDB) EXPECT() *MockproposalDBMockRecorder {
	return m.recorder
}

// AddProposal mocks base method.
func (m *MockproposalDB) AddProposal(arg0 context.Context, arg1 *types.Proposal) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddProposal", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddProposal indicates an expected call of AddProposal.
func (mr *MockproposalDBMockRecorder) AddProposal(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddProposal", reflect.TypeOf((*MockproposalDB)(nil).AddProposal), arg0, arg1)
}

// HasProposal mocks base method.
func (m *MockproposalDB) HasProposal(arg0 types.ProposalID) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasProposal", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasProposal indicates an expected call of HasProposal.
func (mr *MockproposalDBMockRecorder) HasProposal(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasProposal", reflect.TypeOf((*MockproposalDB)(nil).HasProposal), arg0)
}

// MockeligibilityValidator is a mock of eligibilityValidator interface.
type MockeligibilityValidator struct {
	ctrl     *gomock.Controller
	recorder *MockeligibilityValidatorMockRecorder
}

// MockeligibilityValidatorMockRecorder is the mock recorder for MockeligibilityValidator.
type MockeligibilityValidatorMockRecorder struct {
	mock *MockeligibilityValidator
}

// NewMockeligibilityValidator creates a new mock instance.
func NewMockeligibilityValidator(ctrl *gomock.Controller) *MockeligibilityValidator {
	mock := &MockeligibilityValidator{ctrl: ctrl}
	mock.recorder = &MockeligibilityValidatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockeligibilityValidator) EXPECT() *MockeligibilityValidatorMockRecorder {
	return m.recorder
}

// CheckEligibility mocks base method.
func (m *MockeligibilityValidator) CheckEligibility(arg0 context.Context, arg1 *types.Ballot) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckEligibility", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CheckEligibility indicates an expected call of CheckEligibility.
func (mr *MockeligibilityValidatorMockRecorder) CheckEligibility(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckEligibility", reflect.TypeOf((*MockeligibilityValidator)(nil).CheckEligibility), arg0, arg1)
}
