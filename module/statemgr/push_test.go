package statemgr

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock实现IPushStateSyncer接口
type mockPushStateSyncer struct {
	mock.Mock
}

func (m *mockPushStateSyncer) IsReady(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *mockPushStateSyncer) ListLocal(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockPushStateSyncer) Push(ctx context.Context, local string) error {
	args := m.Called(ctx, local)
	return args.Error(0)
}

func TestNewPushStateSyncer(t *testing.T) {
	mockSyncer := &mockPushStateSyncer{}
	worker := NewPushStateSyncer("test", mockSyncer)

	assert.NotNil(t, worker)
	assert.Equal(t, "test", worker.name)
	assert.Equal(t, mockSyncer, worker.syncer)
}

func TestStateSyncerPushWorker_Run_NotReady(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPushStateSyncer{}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(false)

	worker := NewPushStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	// 验证只调用了IsReady，其他方法没有被调用
	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPushWorker_Run_ListLocalError(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPushStateSyncer{}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListLocal", ctx).Return([]string{}, errors.New("list local error"))

	worker := NewPushStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPushWorker_Run_Success(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPushStateSyncer{}

	localItems := []string{"item1", "item2", "item3"}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListLocal", ctx).Return(localItems, nil)
	for _, item := range localItems {
		mockSyncer.On("Push", ctx, item).Return(nil)
	}

	worker := NewPushStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestStateSyncerPushWorker_Run_PushError(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPushStateSyncer{}

	localItems := []string{"item1", "item2"}

	// 设置mock返回值
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListLocal", ctx).Return(localItems, nil)
	mockSyncer.On("Push", ctx, "item1").Return(nil)
	mockSyncer.On("Push", ctx, "item2").Return(errors.New("push error"))

	worker := NewPushStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	mockSyncer.AssertExpectations(t)
}

func TestWithPushLaunchRun(t *testing.T) {
	option := WithPushLaunchRun()
	regInfo := &StateSyncRegInfo{}

	option.apply(regInfo)

	assert.True(t, regInfo.LaunchRun)
}

func TestWithPushSpec(t *testing.T) {
	spec := "0 0 * * *"
	option := WithPushSpec(spec)
	regInfo := &StateSyncRegInfo{}

	option.apply(regInfo)

	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewPushStateSyncRegInfo(t *testing.T) {
	mockSyncer := &mockPushStateSyncer{}
	name := "test-push-syncer"
	spec := "0 0 * * *"

	regInfo := NewPushStateSyncRegInfo(name, mockSyncer, WithPushLaunchRun(), WithPushSpec(spec))

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.True(t, regInfo.LaunchRun)
	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewPushStateSyncRegInfo_DefaultValues(t *testing.T) {
	mockSyncer := &mockPushStateSyncer{}
	name := "test-push-syncer"

	regInfo := NewPushStateSyncRegInfo(name, mockSyncer)

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.False(t, regInfo.LaunchRun)
	assert.Equal(t, "@every 10s", regInfo.Spec)
}

func TestStateSyncerPushWorker_Run_EmptyLocalList(t *testing.T) {
	ctx := context.Background()
	mockSyncer := &mockPushStateSyncer{}

	// 设置mock返回值 - 空的本地列表
	mockSyncer.On("IsReady", ctx).Return(true)
	mockSyncer.On("ListLocal", ctx).Return([]string{}, nil)

	worker := NewPushStateSyncer("test", mockSyncer)
	worker.Run(ctx)

	// 验证没有调用Push方法
	mockSyncer.AssertExpectations(t)
}
