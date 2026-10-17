package dataaccess_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"

	"github.com/stretchr/testify/assert"
)

// TestModel 用于测试的模型
type TestModel struct {
	base.BaseModel
	Name   string `json:"name" gorm:"name"`
	Age    int    `json:"age" gorm:"age"`
	JoinID int    `json:"joinId" gorm:"column:join_id"`
}

func (t *TestModel) TableName() string {
	return "test_model"
}

func (t *TestModel) DbSource() model.DbSource {
	return "main"
}

type TestExpireModel struct {
	base.BaseModel
	base.ExpireAtBaseModel
	Name string `json:"name" gorm:"column:name"`
}

func (t *TestExpireModel) TableName() string {
	return "test_expire_model"
}

func (t *TestExpireModel) DbSource() model.DbSource {
	return "main"
}

// TestJoinModel 用于测试JOIN的模型
type TestJoinModel struct {
	base.BaseModel
	Code string `json:"code" gorm:"column:code"`
	Desc string `json:"desc" gorm:"column:desc"`
}

func (t *TestJoinModel) TableName() string {
	return "test_join_model"
}

func (t *TestJoinModel) DbSource() model.DbSource {
	return "main"
}

func TestDataAccessor(t *testing.T) {

	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}

	// 创建测试数据
	testData := &TestModel{
		Name: "test",
		Age:  18,
	}
	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	ctx := context.Background()

	// 测试 Create
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)
	assert.NotEqual(t, testData.ID, 0)

	// 测试 Get
	result, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, testData.Name, result.Name)
	assert.Equal(t, testData.Age, result.Age)

	// 测试 List
	results, err := accessor.List(ctx, dataaccess.Eq("name", "test"))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, testData.Name, results[0].Name)
	assert.Equal(t, testData.Age, results[0].Age)

	// 测试 Update
	updateData := results[0]
	updateData.Name = "updated"
	updateData.Age = 20
	err = accessor.Update(ctx, updateData)
	assert.NoError(t, err)

	// 验证更新结果
	updated, err := accessor.Get(ctx, testData)
	assert.NoError(t, err)
	assert.Equal(t, updateData.Name, updated.Name)
	assert.Equal(t, updateData.Age, updated.Age)

	// 测试 Save
	saveData := updated
	saveData.Name = "saved"
	saveData.Age = 25
	err = accessor.Save(ctx, saveData)
	assert.NoError(t, err)

	// 验证保存结果
	saved, err := accessor.Get(ctx, saveData)
	assert.NoError(t, err)
	assert.Equal(t, saveData.Name, saved.Name)
	assert.Equal(t, saveData.Age, saved.Age)

	// 测试 Delete
	err = accessor.Delete(ctx, saveData)
	assert.NoError(t, err)

	// 验证删除结果
	_, err = accessor.Get(ctx, saveData)
	assert.Error(t, err)
}

// TestDataAccessor_List 测试列表查询功能
func TestDataAccessor_List(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建多条测试数据
	testData1 := &TestModel{Name: "Alice", Age: 25}
	testData2 := &TestModel{Name: "Bob", Age: 30}
	testData3 := &TestModel{Name: "Charlie", Age: 35}

	err = accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	// 测试无条件查询
	results, err := accessor.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// 测试相等条件查询
	results, err = accessor.List(ctx, dataaccess.Eq("name", "Alice"))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Alice", results[0].Name)

	// 测试大于条件查询
	results, err = accessor.List(ctx, dataaccess.Gt("age", 28))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试大于等于条件查询
	results, err = accessor.List(ctx, dataaccess.Gte("age", 30))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试小于条件查询
	results, err = accessor.List(ctx, dataaccess.Lt("age", 32))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试小于等于条件查询
	results, err = accessor.List(ctx, dataaccess.Lte("age", 30))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试不等于条件查询
	results, err = accessor.List(ctx, dataaccess.Ne("name", "Alice"))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试多个条件组合查询
	results, err = accessor.List(ctx, dataaccess.Gte("age", 25), dataaccess.Lte("age", 30))
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestDataAccessor_First 测试获取第一条记录功能
func TestDataAccessor_First(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	testData := &TestModel{Name: "FirstTest", Age: 40}
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 测试获取第一个匹配的记录
	result, err := accessor.First(ctx, dataaccess.Eq("name", "FirstTest"))
	assert.NoError(t, err)
	assert.Equal(t, "FirstTest", result.Name)
	assert.Equal(t, 40, result.Age)

	// 测试查询不存在的记录
	_, err = accessor.First(ctx, dataaccess.Eq("name", "NotExists"))
	assert.Error(t, err)
	assert.Equal(t, dataaccess.ErrDataNotFound, err)
}

// TestDataAccessor_Count 测试计数功能
func TestDataAccessor_Count(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建多条测试数据
	for i := 0; i < 5; i++ {
		testData := &TestModel{
			Name: fmt.Sprintf("CountTest%d", i),
			Age:  20 + i*5,
		}
		err = accessor.Create(ctx, testData)
		assert.NoError(t, err)
	}

	// 测试无条件计数
	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// 测试带条件计数
	count, err = accessor.Count(ctx, dataaccess.Gt("age", 25))
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestDataAccessor_DeleteById 测试按ID删除功能
func TestDataAccessor_DeleteById(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	testData := &TestModel{Name: "DeleteTest", Age: 45}
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 验证数据存在
	_, err = accessor.Get(ctx, testData)
	assert.NoError(t, err)

	// 按ID删除
	err = accessor.DeleteById(ctx, field.ID(testData.ID))
	assert.NoError(t, err)

	// 验证删除成功
	_, err = accessor.Get(ctx, testData)
	assert.Error(t, err)
}

// TestDataAccessor_GetByID 测试按ID获取功能
func TestDataAccessor_GetByID(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	testData := &TestModel{Name: "GetByIDTest", Age: 50}
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)

	// 按ID获取
	result, err := accessor.GetByID(ctx, field.ID(testData.ID))
	assert.NoError(t, err)
	assert.Equal(t, testData.Name, result.Name)
	assert.Equal(t, testData.Age, result.Age)
	assert.Equal(t, testData.ID, result.ID)

	// 测试获取不存在的ID
	_, err = accessor.GetByID(ctx, field.ID(99999))
	assert.Error(t, err)
}

func TestGormDataAccessor_CreateWithTtlZeroDoesNotExpire(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestExpireModel{})

	accessor := &dataaccess.DataAccessor[*TestExpireModel]{}
	ctx := context.Background()

	record := &TestExpireModel{Name: "ttl-zero-create"}
	err = accessor.CreateWithTtl(ctx, record, 0)
	assert.NoError(t, err)
	assert.Nil(t, record.ExpireAt)

	result, err := accessor.GetByID(ctx, field.ID(record.ID))
	assert.NoError(t, err)
	assert.Equal(t, record.Name, result.Name)
	assert.Nil(t, result.ExpireAt)
}

func TestGormDataAccessor_UpdateWithTtlZeroClearsExpireAtWithOnlyFields(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestExpireModel{})

	accessor := &dataaccess.DataAccessor[*TestExpireModel]{}
	ctx := context.Background()

	record := &TestExpireModel{Name: "ttl-update-before"}
	err = accessor.CreateWithTtl(ctx, record, time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, record.ExpireAt)

	record.Name = "ttl-update-after"
	err = accessor.UpdateWithTtl(ctx, record, 0, dataaccess.OnlyFields("name"))
	assert.NoError(t, err)

	result, err := accessor.GetByID(ctx, field.ID(record.ID))
	assert.NoError(t, err)
	assert.Equal(t, "ttl-update-after", result.Name)
	assert.Nil(t, result.ExpireAt)
}

// TestDataAccessor_Transaction 测试事务功能
func TestDataAccessor_Transaction(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 测试成功的事务
	err = accessor.Transaction(ctx, func(ctx context.Context) error {
		testData1 := &TestModel{Name: "TxTest1", Age: 60}
		testData2 := &TestModel{Name: "TxTest2", Age: 65}

		if err := accessor.Create(ctx, testData1); err != nil {
			return err
		}
		if err := accessor.Create(ctx, testData2); err != nil {
			return err
		}
		return nil
	})
	assert.NoError(t, err)

	// 验证事务中的数据已提交
	results, err := accessor.List(ctx, dataaccess.Like("name", "%TxTest%"))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试失败的事务（回滚）
	initialCount, err := accessor.Count(ctx)
	assert.NoError(t, err)

	err = accessor.Transaction(ctx, func(ctx context.Context) error {
		testData := &TestModel{Name: "TxTestFail", Age: 70}
		if err := accessor.Create(ctx, testData); err != nil {
			return err
		}
		// 故意返回错误以触发回滚
		return errors.New("intentional error")
	})
	assert.Error(t, err)

	// 验证事务已回滚，数据量没有变化
	finalCount, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, initialCount, finalCount)
}

// TestHelperFunctions 测试辅助函数
func TestHelperFunctions(t *testing.T) {
	// 测试 SF 函数（默认主表）
	sf := dataaccess.SF("name")
	result := sf("users")
	assert.Equal(t, "users.name", result)

	// 测试 SF 函数带 JOIN 表
	result = sf("main_table", "join_table1", "join_table2")
	assert.Equal(t, "main_table.name", result)

	// 测试 SFN 函数
	sfn0 := dataaccess.SFN(0, "id")
	result = sfn0("main", "join1", "join2")
	assert.Equal(t, "main.id", result)

	sfn1 := dataaccess.SFN(1, "username")
	result = sfn1("main", "join1", "join2")
	assert.Equal(t, "join1.username", result)

	sfn2 := dataaccess.SFN(2, "email")
	result = sfn2("main", "join1", "join2")
	assert.Equal(t, "join2.email", result)
}

// TestDataAccessor_ListWithOption 测试 ListWithOption 功能
func TestDataAccessor_ListWithOption(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	for i := 0; i < 5; i++ {
		testData := &TestModel{
			Name: fmt.Sprintf("ListOpt%d", i),
			Age:  20 + i*5,
		}
		err = accessor.Create(ctx, testData)
		assert.NoError(t, err)
	}

	var results []*TestModel
	// 测试基本条件查询
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.Eq("name", "ListOpt0"),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "ListOpt0", results[0].Name)

	// 测试多个条件组合
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.Gte("age", 25),
			dataaccess.Lte("age", 35),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// 测试 Limit 和 Offset
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.Order("age", dataaccess.OrderTypeAsc),
			dataaccess.Limit(2),
			dataaccess.Offset(1),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestDataAccessor_ListWithOption_SelectFields 测试 WithSelect 功能
func TestDataAccessor_ListWithOption_SelectFields(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	testData := &TestModel{Name: "SelectTest", Age: 100}
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)

	var results []*TestModel
	// 测试使用 SF 选择字段
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(dataaccess.Eq("name", "SelectTest")),
		dataaccess.WithSelect(
			dataaccess.SF("id"),
			dataaccess.SF("name"),
			dataaccess.SF("age"),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "SelectTest", results[0].Name)
	assert.Equal(t, 100, results[0].Age)
}

// TestDataAccessor_ListWithOption_OrderBy 测试 WithOrder 功能
func TestDataAccessor_ListWithOption_OrderBy(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	for i := 0; i < 3; i++ {
		testData := &TestModel{
			Name: fmt.Sprintf("OrderTest%d", i),
			Age:  30 - i, // 降序年龄
		}
		err = accessor.Create(ctx, testData)
		assert.NoError(t, err)
	}

	var results []*TestModel
	// 测试升序排序
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(dataaccess.Order("age", dataaccess.OrderTypeAsc)),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 28, results[0].Age) // OrderTest2
	assert.Equal(t, 29, results[1].Age) // OrderTest1
	assert.Equal(t, 30, results[2].Age) // OrderTest0

	// 测试降序排序
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(dataaccess.Order("age", dataaccess.OrderTypeDesc)),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 30, results[0].Age)
	assert.Equal(t, 29, results[1].Age)
	assert.Equal(t, 28, results[2].Age)
}

func TestDataAccessor_ListWithOption_OrderCase(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	for _, name := range []string{"todo", "other", "done", "can't", "urgent"} {
		err = accessor.Create(ctx, &TestModel{Name: name})
		assert.NoError(t, err)
	}

	var results []*TestModel
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithOrder(dataaccess.OrderCase("name", "urgent", "can't", "todo", "done")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, "urgent", results[0].Name)
	assert.Equal(t, "can't", results[1].Name)
	assert.Equal(t, "todo", results[2].Name)
	assert.Equal(t, "done", results[3].Name)
	assert.Equal(t, "other", results[4].Name)

	results = nil
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithOrder(dataaccess.OrderCaseDesc("name", "urgent", "can't", "todo", "done")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, "done", results[0].Name)
	assert.Equal(t, "todo", results[1].Name)
	assert.Equal(t, "can't", results[2].Name)
	assert.Equal(t, "urgent", results[3].Name)
	assert.Equal(t, "other", results[4].Name)
}

// TestDataAccessor_AdvancedConditions 测试高级查询条件
func TestDataAccessor_AdvancedConditions(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建测试数据
	testData1 := &TestModel{Name: "Advanced1", Age: 20}
	testData2 := &TestModel{Name: "Advanced2", Age: 25}
	testData3 := &TestModel{Name: "Advanced3", Age: 30}
	testData4 := &TestModel{Name: "Other", Age: 35}

	err = accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData4)
	assert.NoError(t, err)

	// 测试 In 条件
	results, err := accessor.List(ctx, dataaccess.In("name", []string{"Advanced1", "Advanced3"}))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试 NotIn 条件
	results, err = accessor.List(ctx, dataaccess.NotIn("name", []string{"Advanced1", "Advanced3"}))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 测试 Like 条件
	results, err = accessor.List(ctx, dataaccess.Like("name", "%Advanced%"))
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// 测试 NotLike 条件
	results, err = accessor.List(ctx, dataaccess.NotLike("name", "%Advanced%"))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Other", results[0].Name)

	// 测试 Between 条件
	results, err = accessor.List(ctx, dataaccess.Between("age", 22, 32))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 age=25 和 age=30

	// 测试 NotBetween 条件
	results, err = accessor.List(ctx, dataaccess.NotBetween("age", 22, 32))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 应该包含 age=20 和 age=35

	// 测试 IsNull 条件（这里测试不会有结果，因为所有字段都有值）
	results, err = accessor.List(ctx, dataaccess.IsNull("name"))
	assert.NoError(t, err)
	assert.Len(t, results, 0)

	// 测试 IsNotNull 条件
	results, err = accessor.List(ctx, dataaccess.IsNotNull("name"))
	assert.NoError(t, err)
	assert.Len(t, results, 4)
}

// TestDataAccessor_And 测试 And 组合条件
func TestDataAccessor_And(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:", ShowSql: false})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 25})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Bob", Age: 30})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 35})
	assert.NoError(t, err)

	// And(Eq("name","Alice"), Gt("age",30))
	results, err := accessor.List(ctx, dataaccess.And(
		dataaccess.Eq("name", "Alice"),
		dataaccess.Gt("age", 30),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 35, results[0].Age)
}

// TestDataAccessor_Or 测试 Or 组合条件
func TestDataAccessor_Or(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:", ShowSql: false})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 25})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Bob", Age: 30})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Charlie", Age: 35})
	assert.NoError(t, err)

	// Or(Eq("name","Alice"), Eq("name","Bob"))
	results, err := accessor.List(ctx, dataaccess.Or(
		dataaccess.Eq("name", "Alice"),
		dataaccess.Eq("name", "Bob"),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// 混合：And 条件 + Or
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

// TestDataAccessor_OrNested 测试 Or 嵌套 And
func TestDataAccessor_OrNested(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:", ShowSql: false})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 25})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Bob", Age: 30})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Charlie", Age: 35})
	assert.NoError(t, err)

	// Or(And(name=Alice, age=25), And(name=Bob, age=30))
	results, err := accessor.List(ctx, dataaccess.Or(
		dataaccess.And(dataaccess.Eq("name", "Alice"), dataaccess.Eq("age", 25)),
		dataaccess.And(dataaccess.Eq("name", "Bob"), dataaccess.Eq("age", 30)),
	))
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestDataAccessor_Distinct 测试去重查询
func TestDataAccessor_Distinct(t *testing.T) {
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:", ShowSql: false})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	store.RelationDbAutoMigrate(context.Background(), &TestModel{})

	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 25})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Alice", Age: 30})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Bob", Age: 25})
	assert.NoError(t, err)
	err = accessor.Create(ctx, &TestModel{Name: "Bob", Age: 35})
	assert.NoError(t, err)

	// Distinct("name")
	results, err := accessor.List(ctx, dataaccess.Distinct("name"))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // Alice 和 Bob

	// Distinct("age")
	results, err = accessor.List(ctx, dataaccess.Distinct("age"))
	assert.NoError(t, err)
	assert.Len(t, results, 3) // 25, 30, 35

	// Distinct + 条件
	results, err = accessor.List(ctx, dataaccess.Eq("name", "Alice"), dataaccess.Distinct("age"))
	assert.NoError(t, err)
	assert.Len(t, results, 2) // Alice age=25 和 age=30
}

// TestDataAccessor_ListWithOption_Join 测试 ListWithOption 的 JOIN 功能
func TestDataAccessor_ListWithOption_Join(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{}, &TestJoinModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建 TestJoinModel 数据
	join1 := &TestJoinModel{Code: "A", Desc: "Description A"}
	join2 := &TestJoinModel{Code: "B", Desc: "Description B"}
	joinAccessor := &dataaccess.DataAccessor[*TestJoinModel]{}
	err = joinAccessor.Create(ctx, join1)
	assert.NoError(t, err)
	err = joinAccessor.Create(ctx, join2)
	assert.NoError(t, err)

	// 创建 TestModel 数据（关联到 TestJoinModel）
	testData1 := &TestModel{Name: "WithJoinA", Age: 25, JoinID: int(join1.ID)}
	testData2 := &TestModel{Name: "WithJoinB", Age: 30, JoinID: int(join2.ID)}
	testData3 := &TestModel{Name: "NoJoin", Age: 35, JoinID: 0}
	err = accessor.Create(ctx, testData1)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData2)
	assert.NoError(t, err)
	err = accessor.Create(ctx, testData3)
	assert.NoError(t, err)

	var results []*TestModel
	// 测试 LEFT JOIN - 应该返回所有主表记录
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithJoin[*TestJoinModel](dataaccess.JoinOn("join_id", "id")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 3) // LEFT JOIN 返回所有主表记录

	// 测试 INNER JOIN - 只返回有关联记录的
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithJoin[*TestJoinModel](
			dataaccess.JoinOn("join_id", "id"),
			dataaccess.WithJoinType(dataaccess.JoinTypeInner),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 2) // 只有两条有关联

	// 测试带条件的 JOIN
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.EqF(dataaccess.SFN(1, "code"), "A"),
		),
		dataaccess.WithJoin[*TestJoinModel](dataaccess.JoinOn("join_id", "id")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "WithJoinA", results[0].Name)

	// 测试带 Select 和 Order 的 JOIN
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(dataaccess.Order("age", dataaccess.OrderTypeAsc)),
		dataaccess.WithJoin[*TestJoinModel](dataaccess.JoinOn("join_id", "id")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 25, results[0].Age)
	assert.Equal(t, 30, results[1].Age)
	assert.Equal(t, 35, results[2].Age)
}

// TestDataAccessor_ConditionsWithFieldFunc 测试带 FieldFunc 的查询条件
func TestDataAccessor_ConditionsWithFieldFunc(t *testing.T) {
	// 配置数据库连接
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})

	store.RelationDbAutoMigrate(context.Background(), &TestModel{}, &TestJoinModel{})

	// 创建 DataAccessor 实例
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 创建 TestJoinModel 数据
	join1 := &TestJoinModel{Code: "FuncTest", Desc: "Test"}
	joinAccessor := &dataaccess.DataAccessor[*TestJoinModel]{}
	err = joinAccessor.Create(ctx, join1)
	assert.NoError(t, err)

	// 创建 TestModel 数据
	testData := &TestModel{Name: "FuncCondTest", Age: 40, JoinID: int(join1.ID)}
	err = accessor.Create(ctx, testData)
	assert.NoError(t, err)

	var results []*TestModel
	// 测试使用 EqF 带字段函数
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.EqF(dataaccess.SFN(1, "id"), join1.ID),
		),
		dataaccess.WithJoin[*TestJoinModel](dataaccess.JoinOn("join_id", "id")),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "FuncCondTest", results[0].Name)

	// 测试使用 GtF 带字段函数
	err = accessor.ListWithOption(ctx, &results,
		dataaccess.WithConditions(
			dataaccess.GtF(dataaccess.SF("age"), 35),
		),
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "FuncCondTest", results[0].Name)
}
