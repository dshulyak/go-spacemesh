// Code generated by MockGen. DO NOT EDIT.
// Source: ./interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/spacemeshos/go-spacemesh/common/types"
	tortoise "github.com/spacemeshos/go-spacemesh/tortoise"
)

// MockmeshProvider is a mock of meshProvider interface.
type MockmeshProvider struct {
	ctrl     *gomock.Controller
	recorder *MockmeshProviderMockRecorder
}

// MockmeshProviderMockRecorder is the mock recorder for MockmeshProvider.
type MockmeshProviderMockRecorder struct {
	mock *MockmeshProvider
}

// NewMockmeshProvider creates a new mock instance.
func NewMockmeshProvider(ctrl *gomock.Controller) *MockmeshProvider {
	mock := &MockmeshProvider{ctrl: ctrl}
	mock.recorder = &MockmeshProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockmeshProvider) EXPECT() *MockmeshProviderMockRecorder {
	return m.recorder
}

// AddBallot mocks base method.
func (m *MockmeshProvider) AddBallot(arg0 *tortoise.DecodedBallot) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddBallot", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddBallot indicates an expected call of AddBallot.
func (mr *MockmeshProviderMockRecorder) AddBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddBallot", reflect.TypeOf((*MockmeshProvider)(nil).AddBallot), arg0)
}

// AddTXsFromProposal mocks base method.
func (m *MockmeshProvider) AddTXsFromProposal(arg0 context.Context, arg1 types.LayerID, arg2 types.ProposalID, arg3 []types.TransactionID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTXsFromProposal", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddTXsFromProposal indicates an expected call of AddTXsFromProposal.
func (mr *MockmeshProviderMockRecorder) AddTXsFromProposal(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTXsFromProposal", reflect.TypeOf((*MockmeshProvider)(nil).AddTXsFromProposal), arg0, arg1, arg2, arg3)
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

// MockballotDecoder is a mock of ballotDecoder interface.
type MockballotDecoder struct {
	ctrl     *gomock.Controller
	recorder *MockballotDecoderMockRecorder
}

// MockballotDecoderMockRecorder is the mock recorder for MockballotDecoder.
type MockballotDecoderMockRecorder struct {
	mock *MockballotDecoder
}

// NewMockballotDecoder creates a new mock instance.
func NewMockballotDecoder(ctrl *gomock.Controller) *MockballotDecoder {
	mock := &MockballotDecoder{ctrl: ctrl}
	mock.recorder = &MockballotDecoderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockballotDecoder) EXPECT() *MockballotDecoderMockRecorder {
	return m.recorder
}

// DecodeBallot mocks base method.
func (m *MockballotDecoder) DecodeBallot(arg0 *types.Ballot) (*tortoise.DecodedBallot, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DecodeBallot", arg0)
	ret0, _ := ret[0].(*tortoise.DecodedBallot)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DecodeBallot indicates an expected call of DecodeBallot.
func (mr *MockballotDecoderMockRecorder) DecodeBallot(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DecodeBallot", reflect.TypeOf((*MockballotDecoder)(nil).DecodeBallot), arg0)
}
