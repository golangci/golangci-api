// Code generated by MockGen. DO NOT EDIT.
// Source: linter.go

// Package linters is a generated GoMock package.
package linters

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	result "github.com/golangci/golangci-api/pkg/goenvbuild/result"
	result0 "github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	executors "github.com/golangci/golangci-api/pkg/worker/lib/executors"
	reflect "reflect"
)

// MockLinter is a mock of Linter interface
type MockLinter struct {
	ctrl     *gomock.Controller
	recorder *MockLinterMockRecorder
}

// MockLinterMockRecorder is the mock recorder for MockLinter
type MockLinterMockRecorder struct {
	mock *MockLinter
}

// NewMockLinter creates a new mock instance
func NewMockLinter(ctrl *gomock.Controller) *MockLinter {
	mock := &MockLinter{ctrl: ctrl}
	mock.recorder = &MockLinterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockLinter) EXPECT() *MockLinterMockRecorder {
	return m.recorder
}

// Run mocks base method
func (m *MockLinter) Run(ctx context.Context, sg *result.StepGroup, exec executors.Executor) (*result0.Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", ctx, sg, exec)
	ret0, _ := ret[0].(*result0.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Run indicates an expected call of Run
func (mr *MockLinterMockRecorder) Run(ctx, sg, exec interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockLinter)(nil).Run), ctx, sg, exec)
}

// Name mocks base method
func (m *MockLinter) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name
func (mr *MockLinterMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockLinter)(nil).Name))
}
