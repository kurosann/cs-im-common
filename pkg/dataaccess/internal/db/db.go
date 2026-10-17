package db

import (
	"context"
	"fmt"
	"io"

	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	"gorm.io/gorm"
)

var relationDbs = safe.Map[model.DbSource, RelationStore]{}

type RelationStore struct {
	*gorm.DB
	io.Closer
}

func Register(source model.DbSource, db RelationStore) {
	relationDbs.Store(source, db)
}

// IsRegistered 检查指定 source 是否已注册。
func IsRegistered(source model.DbSource) bool {
	_, ok := relationDbs.Load(source)
	return ok
}
func GetAll() map[model.DbSource]RelationStore {
	m := make(map[model.DbSource]RelationStore)
	relationDbs.Range(func(key model.DbSource, value RelationStore) bool {
		m[key] = value
		return true
	})
	return m
}

// Unregister 移除已注册的数据库连接，一般用于 bot 等动态数据源的生命周期管理。
func Unregister(source model.DbSource) RelationStore {
	db, _ := relationDbs.LoadAndDelete(source)
	return db
}

type txKeyType string

func (t txKeyType) Key(source model.DbSource) txKeyType {
	return txKeyType(fmt.Sprintf("%s_%s", t, source))
}

const txKey = txKeyType("tx")

func Get(ctx context.Context, source model.DbSource) *gorm.DB {
	tx, ok := ctx.Value(txKey.Key(source)).(*gorm.DB)
	if ok {
		return tx
	}
	db, ok := relationDbs.Load(source)
	if !ok {
		logger.Panicf(ctx, "%s db source is nil", source)
	}
	return db.WithContext(ctx)
}

func GetWithTx(ctx context.Context, source model.DbSource) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txKey.Key(source)).(*gorm.DB)
	if ok {
		return tx, true
	}
	db, ok := relationDbs.Load(source)
	if !ok {
		logger.Panicf(ctx, "%s db source is nil", source)
	}
	return db.WithContext(ctx), false
}

func StartTx(ctx context.Context, source model.DbSource, tx *gorm.DB) context.Context {
	txCtx, ok := ctx.(*relationTxCtx)
	if ok {
		return txCtx.withValue(txKey.Key(source), tx)
	}
	return newRelationTxCtx(ctx, map[any]any{txKey.Key(source): tx})
}

var _ context.Context = (*relationTxCtx)(nil)

type relationTxCtx struct {
	values map[any]any
	context.Context
}

func newRelationTxCtx(ctx context.Context, defaultValues map[any]any) context.Context {
	if defaultValues == nil {
		defaultValues = make(map[any]any)
	}
	return &relationTxCtx{
		values:  defaultValues,
		Context: ctx,
	}
}

func (t relationTxCtx) Value(key any) any {
	if t.values != nil {
		if v, ok := t.values[key]; ok {
			return v
		}
	}
	return t.Context.Value(key)
}

func (t relationTxCtx) withValue(key, value any) context.Context {
	if t.values == nil {
		t.values = make(map[any]any)
	}
	_, ok := t.values[key]
	if ok {
		return t
	}
	t.values[key] = value
	return t
}
