package model

import (
	"time"
	"vane-common/pkg/dataaccess/field"
)

type DBType string

const (
	DBTypeRelation DBType = "relation"
	DBTypeKv       DBType = "kv"
)

type DbSource string

type Table interface {
	TableName() string
	GetID() field.IID
	SetID(field.IID)
	IDColumn() string
	Create()
	Update()
	DbSource() DbSource
	DbType() DBType
}

type OwnerTable interface {
	GetOwner() string
}
type ExpireAtTable interface {
	GetExpireAt() *time.Time
	SetExpireAt(*time.Time)
	ExpireAtColumn() string
}

// revision 实现同步数据使用save 普通更新使用update
// save 相同revision时,不更新数据
// update 相同revision时,revision会自增然后更新数据,大于revision直接更新数据不自增revision
type RevisionTable interface {
	GetRevision() int64
	SetRevision(revision int64)
	RevisionColumn() string
}
type DynamicSourceTable interface {
	SetDbSource(DbSource)
	SetDbType(DBType)
}

type TableMarshaler interface {
	// 序列化
	Marshal() ([]byte, error)
	// 反序列化
	Unmarshal([]byte) error
	// 反序列化为map 用于list条件判断
	UnmarshalMap([]byte, *map[string]any) error
}
