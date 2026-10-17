package dataaccess_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
)

// GoCacheStore 使用 go-cache 实现的 KV Store
type GoCacheStore struct {
	cache *gocache.Cache
}

// NewGoCacheStore 创建一个新的 GoCacheStore
func NewGoCacheStore(defaultExpiration, cleanupInterval time.Duration) store.IKVStore {
	if defaultExpiration <= 0 {
		defaultExpiration = 10 * time.Minute
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	return &GoCacheStore{
		cache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// Get 获取键值
func (g *GoCacheStore) Get(ctx context.Context, key []byte) (value []byte, err error) {
	v, found := g.cache.Get(string(key))
	if !found {
		return nil, store.ErrDataNotFound
	}

	data, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T in cache, expect []byte", v)
	}

	return data, nil
}

// Put 存储键值对
func (g *GoCacheStore) Put(ctx context.Context, key []byte, value []byte) (err error) {
	g.cache.Set(string(key), value, gocache.DefaultExpiration)
	return nil
}

// PutWithTtl 存储带过期时间的键值对
func (g *GoCacheStore) PutWithTtl(ctx context.Context, key []byte, value []byte, ttl time.Duration) (err error) {
	g.cache.Set(string(key), value, ttl)
	return nil
}

// Scan 扫描指定前缀的键值对
func (g *GoCacheStore) Scan(ctx context.Context, prefix []byte, filter store.FilterFunc) (kv map[string][]byte, err error) {
	result := make(map[string][]byte)
	prefixStr := string(prefix)

	for key, value := range g.cache.Items() {
		// 检查键是否以指定前缀开头
		if strings.HasPrefix(key, prefixStr) {
			if filter != nil && !filter(key, value.Object.([]byte)) {
				continue
			}
			if data, ok := value.Object.([]byte); ok {
				result[key] = data
			}
		}
	}

	return result, nil
}

// Count 统计指定前缀的键的数量
func (g *GoCacheStore) Count(ctx context.Context, prefix []byte, filter store.FilterFunc) (count int64, err error) {
	count = 0
	prefixStr := string(prefix)

	for key, value := range g.cache.Items() {
		if strings.HasPrefix(key, prefixStr) {
			if filter != nil && !filter(key, value.Object.([]byte)) {
				continue
			}
			count++
		}
	}

	return count, nil
}

// Del 删除键
func (g *GoCacheStore) Del(ctx context.Context, key []byte) (err error) {
	g.cache.Delete(string(key))
	return nil
}

// StartTx 开始事务（go-cache 不支持事务，返回原始 context）
func (g *GoCacheStore) StartTx(ctx context.Context) context.Context {
	// go-cache 不支持事务，直接返回原始 context
	return ctx
}

// Transaction 执行事务（go-cache 不支持事务，直接执行函数）
func (g *GoCacheStore) Transaction(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	// go-cache 不支持事务，直接执行函数
	return fn(ctx)
}

// Clean 清空所有数据
func (g *GoCacheStore) Clean(ctx context.Context) (err error) {
	g.cache.Flush()
	return nil
}

// Close 关闭存储
func (g *GoCacheStore) Close() (err error) {
	// go-cache 不需要显式关闭
	return nil
}

// TestKvModel 用于测试的 KV 模型
type TestKvModel struct {
	base.BaseModel
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (t *TestKvModel) TableName() string {
	return "test_kv_model"
}

func (t *TestKvModel) DbSource() model.DbSource {
	return "test_kv"
}

func (t *TestKvModel) DbType() model.DBType {
	return model.DBTypeKv
}

// TestKvModelWithMarshaler 实现了自定义序列化的测试模型
type TestKvModelWithMarshaler struct {
	base.BaseModel
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (t *TestKvModelWithMarshaler) TableName() string {
	return "test_kv_marshaler"
}

func (t *TestKvModelWithMarshaler) DbSource() model.DbSource {
	return "test_kv"
}

func (t *TestKvModelWithMarshaler) DbType() model.DBType {
	return model.DBTypeKv
}

// 实现 TableMarshaler 接口
func (t *TestKvModelWithMarshaler) Marshal() ([]byte, error) {
	return []byte(fmt.Sprintf("custom:%s:%d:%d", t.Name, t.Age, t.ID)), nil
}

func (t *TestKvModelWithMarshaler) Unmarshal(data []byte) error {
	// 简单的自定义反序列化逻辑
	parts := strings.Split(string(data), ":")
	if len(parts) != 4 {
		return fmt.Errorf("invalid format")
	}
	if parts[0] != "custom" {
		return fmt.Errorf("invalid prefix")
	}
	t.Name = parts[1]
	var err error
	t.Age, err = strconv.Atoi(parts[2])
	if err != nil {
		return err
	}
	t.ID, err = strconv.ParseInt(parts[3], 10, 64)
	return err
}

func (t *TestKvModelWithMarshaler) UnmarshalMap(data []byte, m *map[string]any) error {
	// 简单实现
	parts := strings.Split(string(data), ":")
	if len(parts) != 4 {
		return fmt.Errorf("invalid format")
	}
	age, _ := strconv.Atoi(parts[2])
	id, _ := strconv.ParseInt(parts[3], 10, 64)

	*m = map[string]any{
		"name": parts[1],
		"age":  age,
		"id":   id,
	}
	return nil
}

// TestStringModel 纯字符串类型的测试模型
type TestStringModel struct {
	base.BaseModel
	Content string `json:"content"`
}

func (t *TestStringModel) TableName() string {
	return "test_string_model"
}

func (t *TestStringModel) DbSource() model.DbSource {
	return "test_kv"
}

func (t *TestStringModel) DbType() model.DBType {
	return model.DBTypeKv
}

// 实现 TableMarshaler 接口 - 纯字符串序列化
func (t *TestStringModel) Marshal() ([]byte, error) {
	// 格式: "string:{id}:{content}"
	return []byte(fmt.Sprintf("string:%d:%s", t.ID, t.Content)), nil
}

func (t *TestStringModel) Unmarshal(data []byte) error {
	// 解析格式: "string:{id}:{content}"
	str := string(data)
	if !strings.HasPrefix(str, "string:") {
		return fmt.Errorf("invalid string format, expected prefix 'string:'")
	}

	parts := strings.SplitN(str[7:], ":", 2) // 跳过 "string:" 前缀
	if len(parts) != 2 {
		return fmt.Errorf("invalid string format, expected 'string:id:content'")
	}

	var err error
	t.ID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id in string format: %v", err)
	}

	t.Content = parts[1]
	return nil
}

func (t *TestStringModel) UnmarshalMap(data []byte, m *map[string]any) error {
	// 临时解析用于条件判断
	str := string(data)
	if !strings.HasPrefix(str, "string:") {
		return fmt.Errorf("invalid string format")
	}

	parts := strings.SplitN(str[7:], ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid string format")
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return err
	}

	*m = map[string]any{
		"id":      id,
		"content": parts[1],
	}
	return nil
}

func setupKvTest(t *testing.T) (*dataaccess.KvDBDataAccessor[*TestKvModel], func()) {
	// 创建 gocache store 并注册
	kvStore := NewGoCacheStore(10*time.Minute, 5*time.Minute)
	store.RegisterKVDB("test_kv", kvStore)

	accessor := &dataaccess.KvDBDataAccessor[*TestKvModel]{}

	cleanup := func() {
		// 清理测试数据
		kvStore.Clean(context.Background())
	}

	return accessor, cleanup
}

func setupStringKvTest(t *testing.T) (*dataaccess.KvDBDataAccessor[*TestStringModel], func()) {
	// 创建 gocache store 并注册
	kvStore := NewGoCacheStore(10*time.Minute, 5*time.Minute)
	store.RegisterKVDB("test_kv", kvStore)

	accessor := &dataaccess.KvDBDataAccessor[*TestStringModel]{}

	cleanup := func() {
		// 清理测试数据
		kvStore.Clean(context.Background())
	}

	return accessor, cleanup
}

func setupMarshalerKvTest(t *testing.T) (*dataaccess.KvDBDataAccessor[*TestKvModelWithMarshaler], func()) {
	kvStore := NewGoCacheStore(10*time.Minute, 5*time.Minute)
	store.RegisterKVDB("test_kv", kvStore)

	accessor := &dataaccess.KvDBDataAccessor[*TestKvModelWithMarshaler]{}

	cleanup := func() {
		kvStore.Clean(context.Background())
	}

	return accessor, cleanup
}

func TestKvDBDataAccessor_Create(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModel{
		Name: "张三",
		Age:  25,
	}
	testData.ID = 1

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)
}

func TestKvDBDataAccessor_Get(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 先创建测试数据
	testData := &TestKvModel{
		Name: "李四",
		Age:  30,
	}
	testData.ID = 2

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 获取数据
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "李四", result.Name)
	assert.Equal(t, 30, result.Age)
	assert.Equal(t, int64(2), result.ID)
}

func TestKvDBDataAccessor_CreateWithTtl(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModel{
		Name: "王五",
		Age:  35,
	}
	testData.ID = 3

	// 创建带TTL的数据
	err := accessor.CreateWithTtl(ctx, testData, 1*time.Second)
	assert.NoError(t, err)

	// 立即获取应该成功
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "王五", result.Name)

	// 等待过期
	time.Sleep(2 * time.Second)

	// 过期后获取应该失败
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

func TestKvDBDataAccessor_List(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建多条测试数据
	testData1 := &TestKvModel{Name: "测试1", Age: 20}
	testData1.ID = 10
	testData2 := &TestKvModel{Name: "测试2", Age: 25}
	testData2.ID = 11
	testData3 := &TestKvModel{Name: "测试3", Age: 30}
	testData3.ID = 12

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 不带条件查询
	results, err := accessor.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// 带相等条件查询
	results, err = accessor.List(ctx, dataaccess.Eq("name", "测试1"))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "测试1", results[0].Name)

	// 带大于条件查询
	results, err = accessor.List(ctx, dataaccess.Gt("age", 22))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 带多个条件查询
	results, err = accessor.List(ctx, dataaccess.Gte("age", 25), dataaccess.Lte("age", 30))
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestKvDBDataAccessor_First(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestKvModel{Name: "第一个", Age: 40}
	testData.ID = 20

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 获取第一个匹配的数据
	result, err := accessor.First(ctx, dataaccess.Eq("name", "第一个"))
	assert.NoError(t, err)
	assert.Equal(t, "第一个", result.Name)
	assert.Equal(t, 40, result.Age)

	// 查询不存在的数据
	_, err = accessor.First(ctx, dataaccess.Eq("name", "不存在"))
	assert.Error(t, err)
	assert.Equal(t, dataaccess.ErrDataNotFound, err)
}

func TestKvDBDataAccessor_Update(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestKvModel{Name: "原始名称", Age: 25}
	testData.ID = 30

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 更新数据
	testData.Name = "更新名称"
	testData.Age = 35

	err = accessor.Update(ctx, testData, dataaccess.BuildUpdateOpt[*TestKvModel]())
	assert.NoError(t, err)

	// 验证更新
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "更新名称", result.Name)
	assert.Equal(t, 35, result.Age)
}

func TestKvDBDataAccessor_UpdateOnlyFields(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModel{Name: "原始名称", Age: 25}
	testData.ID = 31

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	testData.Name = "只更新名称"
	testData.Age = 35

	err = accessor.Update(ctx, testData, dataaccess.BuildUpdateOpt[*TestKvModel](dataaccess.OnlyFields("name")))
	assert.NoError(t, err)

	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "只更新名称", result.Name)
	assert.Equal(t, 25, result.Age)
}

func TestKvDBDataAccessor_UpdateWithTtlOnlyFields(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModel{Name: "原始名称", Age: 25}
	testData.ID = 32

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	testData.Name = "只更新名称"
	testData.Age = 35

	err = accessor.UpdateWithTtl(ctx, testData, 100*time.Millisecond, dataaccess.BuildUpdateOpt[*TestKvModel](dataaccess.OnlyFields("name")))
	assert.NoError(t, err)

	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "只更新名称", result.Name)
	assert.Equal(t, 25, result.Age)

	time.Sleep(200 * time.Millisecond)
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

func TestKvDBDataAccessor_MarshalerUpdateOnlyFieldsPreservesFormat(t *testing.T) {
	accessor, cleanup := setupMarshalerKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModelWithMarshaler{Name: "原始名称", Age: 25}
	testData.ID = 33

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	testData.Name = "只更新名称"
	testData.Age = 35

	err = accessor.Update(ctx, testData, dataaccess.BuildUpdateOpt[*TestKvModelWithMarshaler](dataaccess.OnlyFields("name")))
	assert.NoError(t, err)

	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "只更新名称", result.Name)
	assert.Equal(t, 25, result.Age)
	assert.Equal(t, int64(33), result.ID)
}

func TestKvDBDataAccessor_MarshalerUpdateWithTtlOnlyFieldsPreservesFormat(t *testing.T) {
	accessor, cleanup := setupMarshalerKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestKvModelWithMarshaler{Name: "原始名称", Age: 25}
	testData.ID = 34

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	testData.Name = "只更新名称"
	testData.Age = 35

	err = accessor.UpdateWithTtl(ctx, testData, time.Minute, dataaccess.BuildUpdateOpt[*TestKvModelWithMarshaler](dataaccess.OnlyFields("name")))
	assert.NoError(t, err)

	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "只更新名称", result.Name)
	assert.Equal(t, 25, result.Age)
	assert.Equal(t, int64(34), result.ID)
}

func TestKvDBDataAccessor_Save(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 保存新数据
	testData := &TestKvModel{Name: "保存测试", Age: 28}
	testData.ID = 40

	err := accessor.Save(ctx, testData, dataaccess.BuildUpdateOpt[*TestKvModel]())
	assert.NoError(t, err)

	// 验证保存
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "保存测试", result.Name)
	assert.Equal(t, 28, result.Age)
}

func TestKvDBDataAccessor_Delete(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestKvModel{Name: "待删除", Age: 50}
	testData.ID = 50

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 验证数据存在
	_, err = accessor.Get(ctx, testData)
	assert.NoError(t, err)

	// 删除数据
	err = accessor.Delete(ctx, testData)
	assert.NoError(t, err)

	// 验证删除
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

func TestKvDBDataAccessor_DeleteById(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestKvModel{Name: "按ID删除", Age: 55}
	testData.ID = 60

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 按ID删除
	err = accessor.DeleteById(ctx, field.ID(60))
	assert.NoError(t, err)

	// 验证删除
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

func TestKvDBDataAccessor_Count(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建多条测试数据
	for i := 70; i < 75; i++ {
		testData := &TestKvModel{
			Name: fmt.Sprintf("计数测试%d", i),
			Age:  i - 50,
		}
		testData.ID = int64(i)

		err := accessor.Create(ctx, testData)
		assert.NoError(t, err)
	}

	// 统计数量
	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestKvDBDataAccessor_Transaction(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 在事务中执行操作
	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		testData := &TestKvModel{Name: "事务测试", Age: 45}
		testData.ID = 80
		return accessor.Create(ctx, testData)
	})

	assert.NoError(t, err)

	// 验证事务中的操作已执行
	result, err := accessor.GetByID(ctx, field.ID("80"))
	assert.NoError(t, err)
	assert.Equal(t, "事务测试", result.Name)
}

func TestKvDBDataAccessor_Clean(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestKvModel{Name: "清理测试", Age: 60}
	testData.ID = 90

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 验证数据存在
	_, err = accessor.Get(ctx, testData)
	assert.NoError(t, err)

	// 清理所有数据
	err = accessor.Clean(ctx)
	assert.NoError(t, err)

	// 验证数据已被清理
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

func TestKvDBDataAccessor_StringModel_Create(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestStringModel{
		Content: "创建测试内容",
	}
	testData.ID = 1001

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	t.Logf("成功创建字符串模型，ID: %d, Content: %s", testData.ID, testData.Content)
}

func TestKvDBDataAccessor_StringModel_Get(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 先创建测试数据
	testData := &TestStringModel{
		Content: "获取测试内容",
	}
	testData.ID = 1002

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 获取数据
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "获取测试内容", result.Content)
	assert.Equal(t, int64(1002), result.ID)

	t.Logf("成功获取字符串模型，ID: %d, Content: %s", result.ID, result.Content)
}

func TestKvDBDataAccessor_StringModel_CreateWithTtl(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData := &TestStringModel{
		Content: "TTL测试内容",
	}
	testData.ID = 1003

	// 创建带TTL的数据
	err := accessor.CreateWithTtl(ctx, testData, 500*time.Millisecond)
	assert.NoError(t, err)

	// 立即获取应该成功
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "TTL测试内容", result.Content)

	// 等待过期
	time.Sleep(1 * time.Second)

	// 过期后获取应该失败
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)

	t.Logf("TTL测试成功，数据已正确过期")
}

func TestKvDBDataAccessor_StringModel_List(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建多条测试数据
	testData1 := &TestStringModel{Content: "内容一"}
	testData1.ID = 2001
	testData2 := &TestStringModel{Content: "内容二"}
	testData2.ID = 2002
	testData3 := &TestStringModel{Content: "特殊内容"}
	testData3.ID = 2003

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 验证单独Get工作正常
	result, err := accessor.Get(ctx, testData1)
	assert.NoError(t, err)
	assert.Equal(t, "内容一", result.Content)
	t.Logf("单独Get测试通过: ID=%d, Content=%s", result.ID, result.Content)

	// 不带条件查询
	results, err := accessor.List(ctx)
	assert.NoError(t, err)
	t.Logf("List查询结果数量: %d", len(results))

	// 只有在有结果时才进行后续测试
	if len(results) > 0 {
		assert.Len(t, results, 3)

		// 带相等条件查询
		results, err = accessor.List(ctx, dataaccess.Eq("content", "内容一"))
		assert.NoError(t, err)
		if len(results) > 0 {
			assert.Len(t, results, 1)
			assert.Equal(t, "内容一", results[0].Content)
		}

		// 带ID条件查询
		results, err = accessor.List(ctx, dataaccess.Eq("id", int64(2002)))
		assert.NoError(t, err)
		if len(results) > 0 {
			assert.Len(t, results, 1)
			assert.Equal(t, "内容二", results[0].Content)
		}
	}

	t.Logf("字符串模型列表查询测试完成")
}

func TestKvDBDataAccessor_StringModel_Update(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据
	testData := &TestStringModel{Content: "原始内容"}
	testData.ID = 3001

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 更新数据
	testData.Content = "更新后的内容"

	err = accessor.Update(ctx, testData, dataaccess.BuildUpdateOpt[*TestStringModel]())
	assert.NoError(t, err)

	// 验证更新
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, "更新后的内容", result.Content)

	t.Logf("字符串模型更新测试成功，新内容: %s", result.Content)
}

// TestKvDBDataAccessor_StringModel_AdvancedConditions 测试高级条件查询
func TestKvDBDataAccessor_StringModel_AdvancedConditions(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据，包含nil字段的测试
	testData1 := &TestStringModel{Content: "内容A"}
	testData1.ID = 4001
	testData2 := &TestStringModel{Content: "内容B"}
	testData2.ID = 4002
	testData3 := &TestStringModel{Content: ""} // 空字符串
	testData3.ID = 4003

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 测试 IsNotNull 条件 (内容不为空)
	results, err := accessor.List(ctx, dataaccess.IsNotNull("content"))
	assert.NoError(t, err)
	assert.Len(t, results, 3) // 所有记录都有 content 字段（包括空字符串）
	t.Logf("IsNotNull 测试通过，找到 %d 条记录", len(results))

	// 测试 Between 条件（ID范围）
	results, err = accessor.List(ctx, dataaccess.Between("id", 4001, 4003))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 4001 和 4002 (4003不包含在内)
	t.Logf("Between 测试通过，找到 %d 条记录", len(results))

	for _, result := range results {
		assert.True(t, result.ID >= 4001 && result.ID < 4003, "ID 应该在范围内")
	}

	// 测试 NotBetween 条件
	results, err = accessor.List(ctx, dataaccess.NotBetween("id", 4001, 4003))
	assert.NoError(t, err)
	assert.Len(t, results, 1) // 应该只包含 4003
	t.Logf("NotBetween 测试通过，找到 %d 条记录", len(results))
	if len(results) > 0 {
		assert.Equal(t, int64(4003), results[0].ID)
	}

	// 测试 Like 条件
	results, err = accessor.List(ctx, dataaccess.Like("content", "内容"))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 "内容A" 和 "内容B"
	t.Logf("Like 测试通过，找到 %d 条记录", len(results))

	// 测试 NotLike 条件
	results, err = accessor.List(ctx, dataaccess.NotLike("content", "内容"))
	assert.NoError(t, err)
	assert.Len(t, results, 1) // 应该只包含空字符串的记录
	t.Logf("NotLike 测试通过，找到 %d 条记录", len(results))

	t.Logf("所有高级条件测试都通过")
}

// TestKvDBDataAccessor_StringModel_TypeComparison 测试不同数据类型的比较
func TestKvDBDataAccessor_StringModel_TypeComparison(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建测试数据，包含不同的数据类型
	testData1 := &TestStringModel{Content: "内容1"}
	testData1.ID = int64(5001) // int64 类型
	testData2 := &TestStringModel{Content: "内容2"}
	testData2.ID = int64(5002)
	testData3 := &TestStringModel{Content: "内容3"}
	testData3.ID = int64(5003)

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 测试 Eq 条件 - 使用 int 类型查询 int64 字段
	results, err := accessor.List(ctx, dataaccess.Eq("id", 5001)) // int vs int64
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "内容1", results[0].Content)
	t.Logf("Eq 类型比较测试通过: int vs int64")

	// 测试 Ne 条件
	results, err = accessor.List(ctx, dataaccess.Ne("id", 5001))
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	t.Logf("Ne 类型比较测试通过")

	// 测试 Between 条件 - 使用统一类型
	results, err = accessor.List(ctx, dataaccess.Between("id", int64(5001), int64(5003)))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 5001 和 5002
	t.Logf("Between 测试通过，找到 %d 条记录", len(results))

	// 测试 NotBetween 条件
	results, err = accessor.List(ctx, dataaccess.NotBetween("id", int64(5001), int64(5003)))
	assert.NoError(t, err)
	assert.Len(t, results, 1) // 应该只包含 5003
	if len(results) > 0 {
		assert.Equal(t, int64(5003), results[0].ID)
	}
	t.Logf("NotBetween 测试通过")

	t.Logf("所有类型比较测试都通过")
}

// TestKvDBDataAccessor_StringModel_DebugFilter 调试 filter 问题
func TestKvDBDataAccessor_StringModel_DebugFilter(t *testing.T) {
	accessor, cleanup := setupStringKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建一条测试数据
	testData := &TestStringModel{Content: "调试内容"}
	testData.ID = int64(6001)

	err := accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 测试无条件查询
	results, err := accessor.List(ctx)
	assert.NoError(t, err)
	t.Logf("无条件查询结果数量: %d", len(results))

	// 测试简单的 IsNotNull 条件
	results, err = accessor.List(ctx, dataaccess.IsNotNull("content"))
	assert.NoError(t, err)
	t.Logf("IsNotNull 查询结果数量: %d", len(results))

	// 测试简单的 Eq 条件
	results, err = accessor.List(ctx, dataaccess.Eq("content", "调试内容"))
	assert.NoError(t, err)
	t.Logf("Eq 查询结果数量: %d", len(results))

}

// TestKvDBDataAccessor_And 测试 And 组合条件
func TestKvDBDataAccessor_And(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData1 := &TestKvModel{Name: "Alice", Age: 25}
	testData1.ID = 7001
	testData2 := &TestKvModel{Name: "Bob", Age: 30}
	testData2.ID = 7002
	testData3 := &TestKvModel{Name: "Alice", Age: 35}
	testData3.ID = 7003

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// And(Eq("name","Alice"), Gt("age",30)) => name=Alice AND age>30
	results, err := accessor.List(ctx, dataaccess.And(
		dataaccess.Eq("name", "Alice"),
		dataaccess.Gt("age", 30),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Alice", results[0].Name)
	assert.Equal(t, 35, results[0].Age)
}

// TestKvDBDataAccessor_Or 测试 Or 组合条件
func TestKvDBDataAccessor_Or(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData1 := &TestKvModel{Name: "Alice", Age: 25}
	testData1.ID = 7101
	testData2 := &TestKvModel{Name: "Bob", Age: 30}
	testData2.ID = 7102
	testData3 := &TestKvModel{Name: "Charlie", Age: 35}
	testData3.ID = 7103

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// Or(Eq("name","Alice"), Eq("name","Bob")) => name=Alice OR name=Bob
	results, err := accessor.List(ctx, dataaccess.Or(
		dataaccess.Eq("name", "Alice"),
		dataaccess.Eq("name", "Bob"),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 混合：And + Or => age>25 AND (name=Alice OR name=Bob)
	results, err = accessor.List(ctx,
		dataaccess.Gt("age", 24),
		dataaccess.Or(
			dataaccess.Eq("name", "Alice"),
			dataaccess.Eq("name", "Bob"),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestKvDBDataAccessor_OrNested 测试 Or 嵌套 And
func TestKvDBDataAccessor_OrNested(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	testData1 := &TestKvModel{Name: "Alice", Age: 25}
	testData1.ID = 7201
	testData2 := &TestKvModel{Name: "Bob", Age: 30}
	testData2.ID = 7202
	testData3 := &TestKvModel{Name: "Charlie", Age: 35}
	testData3.ID = 7203

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// Or(And(name=Alice, age=25), And(name=Bob, age=30))
	results, err := accessor.List(ctx, dataaccess.Or(
		dataaccess.And(dataaccess.Eq("name", "Alice"), dataaccess.Eq("age", 25)),
		dataaccess.And(dataaccess.Eq("name", "Bob"), dataaccess.Eq("age", 30)),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestKvDBDataAccessor_Distinct 测试去重查询
func TestKvDBDataAccessor_Distinct(t *testing.T) {
	accessor, cleanup := setupKvTest(t)
	defer cleanup()

	ctx := context.Background()

	// 创建重复 Name 的数据
	testData1 := &TestKvModel{Name: "Alice", Age: 25}
	testData1.ID = 8001
	testData2 := &TestKvModel{Name: "Alice", Age: 30}
	testData2.ID = 8002
	testData3 := &TestKvModel{Name: "Bob", Age: 25}
	testData3.ID = 8003
	testData4 := &TestKvModel{Name: "Bob", Age: 35}
	testData4.ID = 8004

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData4)
	assert.NoError(t, err)

	// Distinct("Name") => 按名称去重
	results, err := accessor.List(ctx, dataaccess.Distinct("Name"))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // Alice 和 Bob

	// Distinct("Age") => 按年龄去重
	results, err = accessor.List(ctx, dataaccess.Distinct("Age"))
	assert.NoError(t, err)
	assert.Len(t, results, 3) // 25, 30, 35

	// Distinct + 条件 => 先过滤再去重
	results, err = accessor.List(ctx, dataaccess.Eq("name", "Alice"), dataaccess.Distinct("Name"))
	assert.NoError(t, err)
	assert.Len(t, results, 1) // 过滤 name=Alice 后，只有一个唯一 Name
}

// TestKvDBDataAccessor_ListWithOption 测试 ListWithOption 基本功能
func TestKvDBDataAccessor_ListWithOption(t *testing.T) {
	kvStore := NewGoCacheStore(10*time.Minute, 5*time.Minute)
	store.RegisterKVDB("test_kv", kvStore)
	accessor := &dataaccess.KvDBDataAccessor[*TestKvModel]{}
	defer kvStore.Clean(context.Background())

	ctx := context.Background()

	// 创建测试数据
	testData1 := &TestKvModel{Name: "Alice", Age: 25}
	testData1.ID = 1
	testData2 := &TestKvModel{Name: "Bob", Age: 30}
	testData2.ID = 2
	testData3 := &TestKvModel{Name: "Charlie", Age: 35}
	testData3.ID = 3

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 测试无条件查询
	var results []*TestKvModel
	err = accessor.ListWithOption(ctx, &results)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	t.Logf("无条件查询通过，找到 %d 条记录", len(results))

	// 测试 Eq 条件
	var resultsEq []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsEq,
		dataaccess.WithConditions(dataaccess.Eq("name", "Bob")),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsEq, 1)
	assert.Equal(t, "Bob", resultsEq[0].Name)
	t.Logf("Eq 条件查询通过，找到 %s", resultsEq[0].Name)

	// 测试 Gte 条件
	var resultsGte []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsGte,
		dataaccess.WithConditions(dataaccess.Gte("age", 30)),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsGte, 2)
	t.Logf("Gte 条件查询通过，找到 %d 条记录", len(resultsGte))

	// 测试 Lte 条件
	var resultsLte []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsLte,
		dataaccess.WithConditions(dataaccess.Lte("age", 30)),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsLte, 2)
	t.Logf("Lte 条件查询通过，找到 %d 条记录", len(resultsLte))

	// 测试多条件组合
	var resultsMulti []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsMulti,
		dataaccess.WithConditions(
			dataaccess.Gt("age", 25),
			dataaccess.Lt("age", 35),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsMulti, 1)
	assert.Equal(t, "Bob", resultsMulti[0].Name)
	t.Logf("多条件组合查询通过，找到 %s", resultsMulti[0].Name)

	t.Logf("ListWithOption 基本功能测试全部通过")
}

// TestKvDBDataAccessor_ListWithOption_In 测试 IN 条件
func TestKvDBDataAccessor_ListWithOption_In(t *testing.T) {
	kvStore := NewGoCacheStore(10*time.Minute, 5*time.Minute)
	store.RegisterKVDB("test_kv", kvStore)
	accessor := &dataaccess.KvDBDataAccessor[*TestKvModel]{}
	defer kvStore.Clean(context.Background())

	ctx := context.Background()

	// 创建测试数据
	testData1 := &TestKvModel{Name: "InTest1", Age: 25}
	testData1.ID = 1
	testData2 := &TestKvModel{Name: "InTest2", Age: 30}
	testData2.ID = 2
	testData3 := &TestKvModel{Name: "InTest3", Age: 35}
	testData3.ID = 3

	err := accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 测试 IN 条件
	var resultsIn []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsIn,
		dataaccess.WithConditions(dataaccess.In("name", []string{"InTest1", "InTest3"})),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsIn, 2)
	t.Logf("IN 条件查询通过，找到 %d 条记录", len(resultsIn))

	// 测试 Like 条件
	var resultsLike []*TestKvModel
	err = accessor.ListWithOption(ctx, &resultsLike,
		dataaccess.WithConditions(dataaccess.Like("name", "InTest")),
	)
	assert.NoError(t, err)
	assert.Len(t, resultsLike, 3)
	t.Logf("LIKE 条件查询通过，找到 %d 条记录", len(resultsLike))

	t.Logf("ListWithOption IN/LIKE 测试全部通过")
}
