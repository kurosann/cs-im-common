package statemgr

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock实现StateEntity接口
type mockStateEntity struct {
	id string
}

func (m *mockStateEntity) GetID() string {
	return m.id
}

// Mock实现IPullStateSyncer接口
type mockPullStateSyncer struct {
	mock.Mock
}

func (m *mockPullStateSyncer) IsReady(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *mockPullStateSyncer) ListRemote(ctx context.Context) ([]*mockStateEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*mockStateEntity), args.Error(1)
}

func (m *mockPullStateSyncer) ListLocal(ctx context.Context) ([]*mockStateEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*mockStateEntity), args.Error(1)
}

func (m *mockPullStateSyncer) CompareExist(ctx context.Context, remote *mockStateEntity, local *mockStateEntity) bool {
	args := m.Called(ctx, remote, local)
	return args.Bool(0)
}

func (m *mockPullStateSyncer) Create(ctx context.Context, remote *mockStateEntity) error {
	args := m.Called(ctx, remote)
	return args.Error(0)
}

func (m *mockPullStateSyncer) Update(ctx context.Context, remote *mockStateEntity, local *mockStateEntity) error {
	args := m.Called(ctx, remote, local)
	return args.Error(0)
}

func (m *mockPullStateSyncer) Delete(ctx context.Context, local *mockStateEntity) error {
	args := m.Called(ctx, local)
	return args.Error(0)
}

func TestNewPullStateSyncer(t *testing.T) {
	mockSyncer := &mockPullStateSyncer{}
	worker := NewPullStateSyncer("test", mockSyncer)

	assert.NotNil(t, worker)
	assert.Equal(t, "test", worker.name)
	assert.Equal(t, mockSyncer, worker.syncer)
}

func TestStateSyncerPullWorker_Run_NotReady(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(false)

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	// 验证只调用了IsReady，其他方法没有被调用
	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPullWorker_Run_ListRemoteError(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListRemote", ctx).Return([]*mockStateEntity{}, errors.New("remote error"))

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPullWorker_Run_ListLocalError(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	remote := []*mockStateEntity{{id: "1"}}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListRemote", ctx).Return(remote, nil)
	mockSyncer.On("ListLocal", ctx).Return([]*mockStateEntity{}, errors.New("local error"))

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPullWorker_Run_CreateNewItem(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	remote := []*mockStateEntity{{id: "1"}}
	local := []*mockStateEntity{}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListRemote", ctx).Return(remote, nil)
	mockSyncer.On("ListLocal", ctx).Return(local, nil)
	mockSyncer.On("Create", ctx, remote[0]).Return(nil)

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPullWorker_Run_UpdateExistingItem(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	remote := []*mockStateEntity{{id: "1"}}
	local := []*mockStateEntity{{id: "1"}}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListRemote", ctx).Return(remote, nil)
	mockSyncer.On("ListLocal", ctx).Return(local, nil)
	mockSyncer.On("CompareExist", ctx, remote[0], local[0]).Return(true)
	mockSyncer.On("Update", ctx, remote[0], local[0]).Return(nil)

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPullWorker_Run_DeleteLocalItem(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPullStateSyncer{}

	remote := []*mockStateEntity{}
	local := []*mockStateEntity{{id: "1"}}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListRemote", ctx).Return(remote, nil)
	mockSyncer.On("ListLocal", ctx).Return(local, nil)
	mockSyncer.On("Delete", ctx, local[0]).Return(nil)

	worker := NewPullStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestWithPullLaunchRun(t *testing.T) {
	option := WithPullLaunchRun()
	regInfo := &StateSyncRegInfo{}

	option.apply(regInfo)

	assert.True(t, regInfo.LaunchRun)
}

func TestWithPullSpec(t *testing.T) {
	spec := "0 0 * * *"
	option := WithPullSpec(spec)
	regInfo := &StateSyncRegInfo{}

	option.apply(regInfo)

	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewPullStateSyncRegInfo(t *testing.T) {
	mockSyncer := &mockPullStateSyncer{}
	name := "test-syncer"
	spec := "0 0 * * *"

	regInfo := NewPullStateSyncRegInfo(name, mockSyncer, WithPullLaunchRun(), WithPullSpec(spec))

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.True(t, regInfo.LaunchRun)
	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewPullStateSyncRegInfo_DefaultValues(t *testing.T) {
	mockSyncer := &mockPullStateSyncer{}
	name := "test-syncer"

	regInfo := NewPullStateSyncRegInfo(name, mockSyncer)

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.False(t, regInfo.LaunchRun)
	assert.Equal(t, "@every 10s", regInfo.Spec)
}
