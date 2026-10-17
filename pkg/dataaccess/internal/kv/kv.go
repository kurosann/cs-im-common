package kv

import (
	"context"

	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/logger"
)

var kvdbs = make(map[model.DbSource]any)

func Register(source model.DbSource, db any) {
	kvdbs[source] = db
}

func Get(source model.DbSource) any {
	if kvdb, ok := Lookup(source); ok {
		return kvdb
	}
	logger.Panicf(context.Background(), "%s kvdb source is nil", source)
	return nil
}

// Lookup 返回已注册的数据源;未注册或为 nil 时返回 false。
//
// 与 Get 的区别是不 panic:调用方需要把"未注册/不具备该能力"当普通错误处理时用它
// (例如按能力取源——数据源已注册但未必实现某个扩展接口)。
func Lookup(source model.DbSource) (any, bool) {
	kvdb, ok := kvdbs[source]
	if !ok || kvdb == nil {
		return nil, false
	}
	return kvdb, true
}
