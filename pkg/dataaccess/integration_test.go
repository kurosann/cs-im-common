package dataaccess_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 事务测试：覆盖顶层事务 / 嵌套事务 / 回滚 / 通用 Transaction 函数
// ============================================================

// setupRelationDB 初始化 SQLite 内存数据库并注册
func setupRelationDB(t *testing.T) {
	t.Helper()
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{
		DSN:     ":memory:",
		ShowSql: false,
	})
	assert.NoError(t, err)
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	db.AutoMigrate(&TestModel{})
}

// TestTransaction_Commit 顶层事务提交
func TestTransaction_Commit(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		return accessor.Create(ctx, &TestModel{Name: "tx-commit-1", Age: 10})
	})
	assert.NoError(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	result, err := accessor.First(ctx, dataaccess.Eq("name", "tx-commit-1"))
	assert.NoError(t, err)
	assert.Equal(t, "tx-commit-1", result.Name)
}

// TestTransaction_Rollback 顶层事务回滚
func TestTransaction_Rollback(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 先创建一条数据确认 db 正常
	assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "before-rollback", Age: 1}))

	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "should-rollback", Age: 99}))
		return errors.New("intentional rollback")
	})
	assert.Error(t, err)

	// 事务内的数据不应存在
	_, err = accessor.First(ctx, dataaccess.Eq("name", "should-rollback"))
	assert.ErrorIs(t, err, dataaccess.ErrDataNotFound)

	// 事务前的数据应仍在
	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestTransaction_Nested_InnerReusesOuterTx 嵌套事务：内层复用外层 tx
func TestTransaction_Nested_InnerReusesOuterTx(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 外层事务
	err := accessor.Transaction(ctx, func(outerCtx context.Context) error {
		assert.NoError(t, accessor.Create(outerCtx, &TestModel{Name: "outer", Age: 1}))

		// 内层"事务"——应该复用外层 tx，不开启新事务
		err := accessor.Transaction(outerCtx, func(innerCtx context.Context) error {
			return accessor.Create(innerCtx, &TestModel{Name: "inner", Age: 2})
		})
		assert.NoError(t, err)

		// 验证在同一个事务中能看到两条数据（通过 count）
		count, err := accessor.Count(outerCtx)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)

		return nil
	})
	assert.NoError(t, err)

	// 提交后两条数据都持久化
	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// TestTransaction_Nested_RollbackFromInner 嵌套事务：内层返回 error 导致外层回滚
func TestTransaction_Nested_RollbackFromInner(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 事务前创建一条
	assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "baseline", Age: 0}))

	err := accessor.Transaction(ctx, func(outerCtx context.Context) error {
		assert.NoError(t, accessor.Create(outerCtx, &TestModel{Name: "outer-before", Age: 1}))

		// 内层返回 error——因为共享同一个 tx，整个事务回滚
		innerErr := accessor.Transaction(outerCtx, func(innerCtx context.Context) error {
			assert.NoError(t, accessor.Create(innerCtx, &TestModel{Name: "inner-fail", Age: 2}))
			return errors.New("inner error")
		})
		return innerErr // 传播内层 error
	})
	assert.Error(t, err)

	// 事务内的数据全部回滚
	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count) // 只有 baseline

	_, err = accessor.First(ctx, dataaccess.Eq("name", "outer-before"))
	assert.ErrorIs(t, err, dataaccess.ErrDataNotFound)
}

// TestTransaction_GeneralFunction_Basic 通用 Transaction 函数：顶层事务
func TestTransaction_GeneralFunction_Basic(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	var tModel TestModel
	err := dataaccess.Transaction(ctx, tModel.DbType(), tModel.DbSource(), func(ctx context.Context) error {
		return accessor.Create(ctx, &TestModel{Name: "general-tx", Age: 42})
	})
	assert.NoError(t, err)

	result, err := accessor.First(ctx, dataaccess.Eq("name", "general-tx"))
	assert.NoError(t, err)
	assert.Equal(t, 42, result.Age)
}

// TestTransaction_GeneralFunction_Rollback 通用 Transaction 函数：回滚
func TestTransaction_GeneralFunction_Rollback(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "keep", Age: 1}))

	var tModel TestModel
	err := dataaccess.Transaction(ctx, tModel.DbType(), tModel.DbSource(), func(ctx context.Context) error {
		assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "gone", Age: 99}))
		return fmt.Errorf("rollback now")
	})
	assert.Error(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	_, err = accessor.First(ctx, dataaccess.Eq("name", "gone"))
	assert.ErrorIs(t, err, dataaccess.ErrDataNotFound)
}

// TestTransaction_GeneralFunction_Nested 通用 Transaction 嵌套
func TestTransaction_GeneralFunction_Nested(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	var tModel TestModel
	err := dataaccess.Transaction(ctx, tModel.DbType(), tModel.DbSource(), func(outerCtx context.Context) error {
		assert.NoError(t, accessor.Create(outerCtx, &TestModel{Name: "outer-g", Age: 1}))

		// 内层通过通用 Transaction 再调——复用外层 tx
		return dataaccess.Transaction(outerCtx, tModel.DbType(), tModel.DbSource(), func(innerCtx context.Context) error {
			return accessor.Create(innerCtx, &TestModel{Name: "inner-g", Age: 2})
		})
	})
	assert.NoError(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// TestTransaction_Mixed_NestedTypes 混合嵌套：DataAccessor.Transaction 与通用 Transaction 互相嵌套
func TestTransaction_Mixed_NestedTypes(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	var tModel TestModel

	// accessor.Transaction 内嵌通用 Transaction
	err := accessor.Transaction(ctx, func(outerCtx context.Context) error {
		assert.NoError(t, accessor.Create(outerCtx, &TestModel{Name: "mix-1", Age: 1}))

		return dataaccess.Transaction(outerCtx, tModel.DbType(), tModel.DbSource(), func(innerCtx context.Context) error {
			return accessor.Create(innerCtx, &TestModel{Name: "mix-2", Age: 2})
		})
	})
	assert.NoError(t, err)

	// 通用 Transaction 内嵌 accessor.Transaction
	err = dataaccess.Transaction(ctx, tModel.DbType(), tModel.DbSource(), func(outerCtx context.Context) error {
		assert.NoError(t, accessor.Create(outerCtx, &TestModel{Name: "mix-3", Age: 3}))

		return accessor.Transaction(outerCtx, func(innerCtx context.Context) error {
			return accessor.Create(innerCtx, &TestModel{Name: "mix-4", Age: 4})
		})
	})
	assert.NoError(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

// TestTransaction_MultipleCreatesInSingleTx 单事务内多次 Create
func TestTransaction_MultipleCreatesInSingleTx(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		for i := range 5 {
			if err := accessor.Create(ctx, &TestModel{Name: fmt.Sprintf("batch-%d", i), Age: i}); err != nil {
				return err
			}
		}
		return nil
	})
	assert.NoError(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

// TestTransaction_UpdateInsideTx 事务内 Update
func TestTransaction_UpdateInsideTx(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	// 先创建
	assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "pre-update", Age: 10}))

	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		record, err := accessor.First(ctx, dataaccess.Eq("name", "pre-update"))
		if err != nil {
			return err
		}
		record.Name = "post-update"
		record.Age = 99
		return accessor.Update(ctx, record)
	})
	assert.NoError(t, err)

	result, err := accessor.First(ctx, dataaccess.Eq("name", "post-update"))
	assert.NoError(t, err)
	assert.Equal(t, 99, result.Age)
}

// TestTransaction_DeleteInsideTx 事务内 Delete
func TestTransaction_DeleteInsideTx(t *testing.T) {
	setupRelationDB(t)
	accessor := &dataaccess.DataAccessor[*TestModel]{}
	ctx := context.Background()

	assert.NoError(t, accessor.Create(ctx, &TestModel{Name: "to-delete", Age: 1}))

	err := accessor.Transaction(ctx, func(ctx context.Context) error {
		record, err := accessor.First(ctx, dataaccess.Eq("name", "to-delete"))
		if err != nil {
			return err
		}
		return accessor.Delete(ctx, record)
	})
	assert.NoError(t, err)

	_, err = accessor.First(ctx, dataaccess.Eq("name", "to-delete"))
	assert.ErrorIs(t, err, dataaccess.ErrDataNotFound)
}

// TestTransaction_KV_Nested 覆盖 KV 事务路径（GoCacheStore 不支持真正事务，验证嵌套不报错）
func TestTransaction_KV_Nested(t *testing.T) {
	kvStore := NewGoCacheStore(0, 0)
	store.RegisterKVDB("test_kv", kvStore)
	defer kvStore.Clean(context.Background())

	accessor := &dataaccess.KvDBDataAccessor[*TestKvModel]{}
	ctx := context.Background()

	// 外层事务
	err := accessor.Transaction(ctx, func(outerCtx context.Context) error {
		kvOuter := &TestKvModel{Name: "kv-outer", Age: 1}
		kvOuter.ID = 9101
		assert.NoError(t, accessor.Create(outerCtx, kvOuter))

		// 内层"事务"——直接执行（GoCache 不支持事务，但代码路径应正常）
		return accessor.Transaction(outerCtx, func(innerCtx context.Context) error {
			kvInner := &TestKvModel{Name: "kv-inner", Age: 2}
			kvInner.ID = 9102
			return accessor.Create(innerCtx, kvInner)
		})
	})
	assert.NoError(t, err)

	count, err := accessor.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// TestTransaction_KV_GeneralFunction 通用 Transaction 走 KV 分支
func TestTransaction_KV_GeneralFunction(t *testing.T) {
	kvStore := NewGoCacheStore(0, 0)
	store.RegisterKVDB("test_kv", kvStore)
	defer kvStore.Clean(context.Background())

	accessor := &dataaccess.KvDBDataAccessor[*TestKvModel]{}
	ctx := context.Background()

	kvData := &TestKvModel{Name: "kv-general", Age: 88}
	kvData.ID = 9201

	var m TestKvModel
	err := dataaccess.Transaction(ctx, m.DbType(), m.DbSource(), func(ctx context.Context) error {
		return accessor.Create(ctx, kvData)
	})
	assert.NoError(t, err)

	result, err := accessor.First(ctx, dataaccess.Eq("name", "kv-general"))
	assert.NoError(t, err)
	assert.Equal(t, 88, result.Age)
}

// TestTransaction_UnsupportedDBType 不支持的 DBType
func TestTransaction_UnsupportedDBType(t *testing.T) {
	ctx := context.Background()
	err := dataaccess.Transaction(ctx, model.DBType("unknown"), model.DbSource("x"), func(ctx context.Context) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
