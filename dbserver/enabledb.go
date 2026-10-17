package dbserver

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"vane-common/module"
	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/logger"
	"vane-common/server"
	pkgJob "vane-common/server/pkg/job"
	"vane-common/server/pkg/role"
)

var _ server.IDBServer = (*MainDBServer)(nil)
var _ server.IServer = (*MainDBServer)(nil)

type MainDBServer struct {
	initDb   func() Db
	dbConfig Db
	cron     *cron.Cron
}

type Db struct {
	DbRelation             map[model.DbSource]store.RelationStore
	DbStore                map[model.DbSource]store.IKVStore
	DisableAutoCreateTable bool
}

func EnableDBServer(f func() Db) *MainDBServer {
	return &MainDBServer{
		initDb: f,
		cron:   cron.New(),
	}
}

func (s *MainDBServer) Init(ctx context.Context) error {
	s.dbConfig = s.initDb()

	for sourceName, relationDb := range s.dbConfig.DbRelation {
		store.RegisterRelationDB(sourceName, relationDb)
	}
	for sourceName, kvDb := range s.dbConfig.DbStore {
		store.RegisterKVDB(sourceName, kvDb)
	}
	return nil
}

func (s *MainDBServer) Reload(ctx context.Context) error {
	s.dbConfig = s.initDb()

	for sourceName, relationDb := range s.dbConfig.DbRelation {
		store.RegisterRelationDB(sourceName, relationDb)
	}
	for sourceName, kvDb := range s.dbConfig.DbStore {
		store.RegisterKVDB(sourceName, kvDb)
	}
	return nil
}

func (s *MainDBServer) Start(ctx context.Context) error {
	s.cron.Start()
	return nil
}

func (s *MainDBServer) Stop() {
	s.cron.Stop()
}

func (s *MainDBServer) InitDB(ctx context.Context, regModels ...module.ModelRegInfo) error {
	if s.dbConfig.DisableAutoCreateTable {
		return nil
	}
	relationModel := make([]model.Table, 0, len(regModels))
	kvModel := make([]model.Table, 0, len(regModels))
	for _, item := range regModels {
		switch item.Model.DbType() {
		case model.DBTypeRelation:
			relationModel = append(relationModel, item.Model)
		case model.DBTypeKv:
			kvModel = append(kvModel, item.Model)
		}
	}
	if len(relationModel) > 0 {
		err := store.RelationDbAutoMigrate(ctx, relationModel...)
		if err != nil {
			logger.Error(ctx, "auto migrate relation db error", zap.Error(err))
			return err
		}
		s.cron.AddFunc("@every 1d", func() {
			state := pkgJob.State{
				IsGlobal:        true,
				ReplicationRole: role.GetRole().ReplicationRole,
				LockTtl:         24 * time.Hour,
			}
			pkgJob.RunJob(ctx, "dbserver", "clean_relation_db", state, func(ctx context.Context) {
				err := store.RelationDbClean(ctx, relationModel...)
				if err != nil {
					logger.Error(ctx, "clean relation db error", zap.Error(err))
				}
			})
		})
	}
	if len(kvModel) > 0 {
		// store.KVDBAutoMigrate(ctx, kvModel...)
	}
	for _, item := range regModels {
		if item.Listen != nil {
			dataaccess.RegisterDataListen(item.Model, item.Listen.Listen)
		}
		if item.CacheTtl != 0 {
			dataaccess.EnableCache(item.Model, item.CacheTtl)
		}
	}
	return nil
}
