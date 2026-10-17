package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"vane-common/pkg/clock"
	"vane-common/pkg/dataaccess/internal/db"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/safe"
)

type RelationStore db.RelationStore

type BeforeRelationAutoMigrater interface {
	BeforeRelationAutoMigrate(ctx context.Context, db *gorm.DB) error
}

func RegisterRelationDB(source model.DbSource, relationDb RelationStore) {
	db.Register(source, db.RelationStore(relationDb))
}

// IsRegistered 检查指定 source 是否已注册。
func IsRegistered(source model.DbSource) bool {
	return db.IsRegistered(source)
}

// UnregisterRelationDB 移除已注册的数据库连接，与 RegisterRelationDB 配对使用。
func UnregisterRelationDB(source model.DbSource) RelationStore {
	return RelationStore(db.Unregister(source))
}
func GetAllRelationDB() []RelationStore {
	var relationDBs []RelationStore
	for _, relationDB := range db.GetAll() {
		relationDBs = append(relationDBs, RelationStore(relationDB))
	}
	return relationDBs
}
func UnregisterAllRelationDB() map[model.DbSource]RelationStore {
	relationDBs := make(map[model.DbSource]RelationStore)
	for source := range db.GetAll() {
		relationDB := UnregisterRelationDB(source)
		relationDBs[source] = relationDB
	}
	return relationDBs
}

func RelationDbAutoMigrate(ctx context.Context, models ...model.Table) error {
	var errs []error
	for _, model := range models {
		gormDB := db.Get(ctx, model.DbSource())
		if before, ok := any(model).(BeforeRelationAutoMigrater); ok {
			if err := before.BeforeRelationAutoMigrate(ctx, gormDB); err != nil {
				errs = append(errs, fmt.Errorf("before auto migrate %s: %w", model.TableName(), err))
				continue
			}
		}
		err := gormDB.AutoMigrate(model)
		if err != nil {
			errs = append(errs, fmt.Errorf("auto migrate %s: %w", model.TableName(), err))
		}
	}
	return errors.Join(errs...)
}
func RelationDbClean(ctx context.Context, models ...model.Table) error {
	wg := safe.NewWaitGroupContext(ctx)
	for _, m := range models {
		if expireAtTable, ok := any(m).(model.ExpireAtTable); ok {
			column := expireAtTable.ExpireAtColumn()
			if column == "" {
				continue
			}
			wg.GoE(func() error {
				return db.Get(ctx, m.DbSource()).Table(m.TableName()).Where("("+column+" IS NOT NULL AND "+column+" < ?)", clock.Now()).Delete(m).Error
			})
		}
	}
	return wg.WaitAndRecover()
}
