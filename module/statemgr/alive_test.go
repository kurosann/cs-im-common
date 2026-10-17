package statemgr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock实现AliveEntity接口
type mockAliveEntity struct {
	timestamp int64
}

func (m *mockAliveEntity) GetTimestamp() int64 {
	return m.timestamp
}

var _ AliveEntity = (*mockAliveEntity)(nil)

var _ IAliveChecker[*mockAliveEntity] = (*mockAliveChecker)(nil)

// Mock实现IAliveChecker接口
type mockAliveChecker struct {
	mock.Mock
}

func (m *mockAliveChecker) IsReady(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func (m *mockAliveChecker) List(ctx context.Context) ([]*mockAliveEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*mockAliveEntity), args.Error(1)
}

func (m *mockAliveChecker) Delete(ctx context.Context, item *mockAliveEntity) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func TestNewAliveChecker(t *testing.T) {
	mockChecker := &mockAliveChecker{}
	ttl := 30 * time.Second
	worker := NewAliveChecker("test", ttl, IAliveChecker[*mockAliveEntity](mockChecker))

	assert.NotNil(t, worker)
	assert.Equal(t, "test", worker.name)
	assert.Equal(t, ttl, worker.ttl)
	assert.Equal(t, mockChecker, worker.checker)
}

func TestAliveCheckerWorker_Run_NotReady(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	// 设置mock返回值
	mockChecker.On("IsReady", ctx).Return(false)

	worker := NewAliveChecker("test", 30*time.Second, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	// 验证只调用了IsReady，其他方法没有被调用
	mockChecker.AssertExpectations(t)
}

func TestAliveCheckerWorker_Run_ListError(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	// 设置mock返回值
	mockChecker.On("IsReady", ctx).Return(true)
	mockChecker.On("List", ctx).Return([]*mockAliveEntity{}, errors.New("list error"))

	worker := NewAliveChecker("test", 30*time.Second, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	mockChecker.AssertExpectations(t)
}

func TestAliveCheckerWorker_Run_DeleteExpiredItems(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	now := time.Now().Unix()
	ttl := 30 * time.Second

	// 创建一个过期的和一个未过期的实体
	expiredItem := &mockAliveEntity{timestamp: now - 60} // 60秒前，超过30秒TTL
	validItem := &mockAliveEntity{timestamp: now - 10}   // 10秒前，未超过30秒TTL

	items := []*mockAliveEntity{expiredItem, validItem}

	// 设置mock返回值
	mockChecker.On("IsReady", ctx).Return(true)
	mockChecker.On("List", ctx).Return(items, nil)
	mockChecker.On("Delete", ctx, expiredItem).Return(nil)

	worker := NewAliveChecker("test", ttl, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	mockChecker.AssertExpectations(t)
}

func TestAliveCheckerWorker_Run_DeleteError(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	now := time.Now().Unix()
	ttl := 30 * time.Second

	// 创建一个过期的实体
	expiredItem := &mockAliveEntity{timestamp: now - 60} // 60秒前，超过30秒TTL
	items := []*mockAliveEntity{expiredItem}

	// 设置mock返回值
	mockChecker.On("IsReady", ctx).Return(true)
	mockChecker.On("List", ctx).Return(items, nil)
	mockChecker.On("Delete", ctx, expiredItem).Return(errors.New("delete error"))

	worker := NewAliveChecker("test", ttl, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	mockChecker.AssertExpectations(t)
}

func TestAliveCheckerWorker_Run_NoExpiredItems(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	now := time.Now().Unix()
	ttl := 30 * time.Second

	// 创建都未过期的实体
	validItem1 := &mockAliveEntity{timestamp: now - 10}
	validItem2 := &mockAliveEntity{timestamp: now - 20}
	items := []*mockAliveEntity{validItem1, validItem2}

	// 设置mock返回值
	mockChecker.On("IsReady", ctx).Return(true)
	mockChecker.On("List", ctx).Return(items, nil)

	worker := NewAliveChecker("test", ttl, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	// 验证没有调用Delete方法
	mockChecker.AssertExpectations(t)
}

func TestWithAliveTtl(t *testing.T) {
	ttl := 60 * time.Second
	option := WithAliveTtl(ttl)
	regInfo := &aliveRegInfo{}

	option.apply(regInfo)

	assert.Equal(t, ttl, regInfo.Ttl)
}

func TestWithAliveLaunchRun(t *testing.T) {
	option := WithAliveLaunchRun()
	regInfo := &aliveRegInfo{}

	option.apply(regInfo)

	assert.True(t, regInfo.LaunchRun)
}

func TestWithAliveSpec(t *testing.T) {
	spec := "0 0 * * *"
	option := WithAliveSpec(spec)
	regInfo := &aliveRegInfo{}

	option.apply(regInfo)

	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewAliveCheckerRegInfo(t *testing.T) {
	mockChecker := &mockAliveChecker{}
	name := "test-alive-checker"
	spec := "0 0 * * *"
	ttl := 60 * time.Second

	regInfo := NewAliveCheckerRegInfo(name, IAliveChecker[*mockAliveEntity](mockChecker),
		WithAliveLaunchRun(),
		WithAliveSpec(spec),
		WithAliveTtl(ttl))

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.True(t, regInfo.LaunchRun)
	assert.Equal(t, spec, regInfo.Spec)
}

func TestNewAliveCheckerRegInfo_DefaultValues(t *testing.T) {
	mockChecker := &mockAliveChecker{}
	name := "test-alive-checker"

	regInfo := NewAliveCheckerRegInfo(name, IAliveChecker[*mockAliveEntity](mockChecker))

	assert.Equal(t, name, regInfo.Name)
	assert.NotNil(t, regInfo.Syncer)
	assert.False(t, regInfo.LaunchRun)
	assert.Equal(t, "@every 10s", regInfo.Spec)
}

func TestAliveCheckerWorker_Run_EmptyList(t *testing.T) {
	ctx := context.Background()
	mockChecker := &mockAliveChecker{}

	// 设置mock返回值 - 空列表
	mockChecker.On("IsReady", ctx).Return(true)
	mockChecker.On("List", ctx).Return([]*mockAliveEntity{}, nil)

	worker := NewAliveChecker("test", 30*time.Second, IAliveChecker[*mockAliveEntity](mockChecker))
	worker.Run(ctx)

	// 验证没有调用Delete方法
	mockChecker.AssertExpectations(t)
}
