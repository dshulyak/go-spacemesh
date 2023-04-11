// Code generated by MockGen. DO NOT EDIT.
// Source: ./nipost.go

// Package activation is a generated GoMock package.
package activation

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	types "github.com/spacemeshos/go-spacemesh/common/types"
)

// MockPoetProvingServiceClient is a mock of PoetProvingServiceClient interface.
type MockPoetProvingServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockPoetProvingServiceClientMockRecorder
}

// MockPoetProvingServiceClientMockRecorder is the mock recorder for MockPoetProvingServiceClient.
type MockPoetProvingServiceClientMockRecorder struct {
	mock *MockPoetProvingServiceClient
}

// NewMockPoetProvingServiceClient creates a new mock instance.
func NewMockPoetProvingServiceClient(ctrl *gomock.Controller) *MockPoetProvingServiceClient {
	mock := &MockPoetProvingServiceClient{ctrl: ctrl}
	mock.recorder = &MockPoetProvingServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPoetProvingServiceClient) EXPECT() *MockPoetProvingServiceClientMockRecorder {
	return m.recorder
}

// PoetServiceID mocks base method.
func (m *MockPoetProvingServiceClient) PoetServiceID(arg0 context.Context) (types.PoetServiceID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PoetServiceID", arg0)
	ret0, _ := ret[0].(types.PoetServiceID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PoetServiceID indicates an expected call of PoetServiceID.
func (mr *MockPoetProvingServiceClientMockRecorder) PoetServiceID(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PoetServiceID", reflect.TypeOf((*MockPoetProvingServiceClient)(nil).PoetServiceID), arg0)
}

// PowParams mocks base method.
func (m *MockPoetProvingServiceClient) PowParams(ctx context.Context) (*PoetPowParams, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PowParams", ctx)
	ret0, _ := ret[0].(*PoetPowParams)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PowParams indicates an expected call of PowParams.
func (mr *MockPoetProvingServiceClientMockRecorder) PowParams(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PowParams", reflect.TypeOf((*MockPoetProvingServiceClient)(nil).PowParams), ctx)
}

// Proof mocks base method.
func (m *MockPoetProvingServiceClient) Proof(ctx context.Context, roundID string) (*types.PoetProofMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Proof", ctx, roundID)
	ret0, _ := ret[0].(*types.PoetProofMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Proof indicates an expected call of Proof.
func (mr *MockPoetProvingServiceClientMockRecorder) Proof(ctx, roundID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Proof", reflect.TypeOf((*MockPoetProvingServiceClient)(nil).Proof), ctx, roundID)
}

// Submit mocks base method.
func (m *MockPoetProvingServiceClient) Submit(ctx context.Context, prefix, challenge []byte, signature types.EdSignature, nodeID types.NodeID, pow PoetPoW) (*types.PoetRound, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Submit", ctx, prefix, challenge, signature, nodeID, pow)
	ret0, _ := ret[0].(*types.PoetRound)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Submit indicates an expected call of Submit.
func (mr *MockPoetProvingServiceClientMockRecorder) Submit(ctx, prefix, challenge, signature, nodeID, pow interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Submit", reflect.TypeOf((*MockPoetProvingServiceClient)(nil).Submit), ctx, prefix, challenge, signature, nodeID, pow)
}

// MockpoetDbAPI is a mock of poetDbAPI interface.
type MockpoetDbAPI struct {
	ctrl     *gomock.Controller
	recorder *MockpoetDbAPIMockRecorder
}

// MockpoetDbAPIMockRecorder is the mock recorder for MockpoetDbAPI.
type MockpoetDbAPIMockRecorder struct {
	mock *MockpoetDbAPI
}

// NewMockpoetDbAPI creates a new mock instance.
func NewMockpoetDbAPI(ctrl *gomock.Controller) *MockpoetDbAPI {
	mock := &MockpoetDbAPI{ctrl: ctrl}
	mock.recorder = &MockpoetDbAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockpoetDbAPI) EXPECT() *MockpoetDbAPIMockRecorder {
	return m.recorder
}

// GetProof mocks base method.
func (m *MockpoetDbAPI) GetProof(arg0 types.PoetProofRef) (*types.PoetProof, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProof", arg0)
	ret0, _ := ret[0].(*types.PoetProof)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetProof indicates an expected call of GetProof.
func (mr *MockpoetDbAPIMockRecorder) GetProof(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProof", reflect.TypeOf((*MockpoetDbAPI)(nil).GetProof), arg0)
}

// ValidateAndStore mocks base method.
func (m *MockpoetDbAPI) ValidateAndStore(ctx context.Context, proofMessage *types.PoetProofMessage) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateAndStore", ctx, proofMessage)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidateAndStore indicates an expected call of ValidateAndStore.
func (mr *MockpoetDbAPIMockRecorder) ValidateAndStore(ctx, proofMessage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateAndStore", reflect.TypeOf((*MockpoetDbAPI)(nil).ValidateAndStore), ctx, proofMessage)
}
