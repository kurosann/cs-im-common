package base

import (
	"time"

	"vane-common/pkg/clock"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/model"
)

type BaseModel struct {
	ID       int64     `json:"id,omitempty" gorm:"column:id;primary_key;auto_increment"`
	CreateBy string    `json:"create_by,omitempty" gorm:"column:create_by"`
	CreateAt time.Time `json:"create_at,omitempty" gorm:"column:create_at;autoCreateTime"`
	UpdateBy string    `json:"update_by,omitempty" gorm:"column:update_by"`
	UpdateAt time.Time `json:"update_at,omitempty" gorm:"column:update_at;autoUpdateTime"`
}

func (t *BaseModel) GetID() field.IID {
	return field.ID(t.ID)
}

func (t *BaseModel) SetID(id field.IID) {
	t.ID = id.Val().(int64)
}

func (t *BaseModel) IDColumn() string {
	return "id"
}

func (t *BaseModel) Create() {
	t.CreateAt = clock.Now()
	t.UpdateAt = clock.Now()
}

func (t *BaseModel) Update() {
	t.UpdateAt = clock.Now()
}

func (t *BaseModel) DbSource() model.DbSource {
	return model.DbSource("main")
}

func (t *BaseModel) DbType() model.DBType {
	return model.DBTypeRelation
}

type ExpireAtBaseModel struct {
	ExpireAt *time.Time `json:"expire_at,omitempty" gorm:"column:expire_at;index"`
}

func (t *ExpireAtBaseModel) GetExpireAt() *time.Time {
	return t.ExpireAt
}

func (t *ExpireAtBaseModel) SetExpireAt(expireAt *time.Time) {
	t.ExpireAt = expireAt
}

func (t *ExpireAtBaseModel) ExpireAtColumn() string {
	return "expire_at"
}

type RevisionBaseModel struct {
	Revision int64 `json:"revision,omitempty" gorm:"column:revision;default:0"`
}

func (t *RevisionBaseModel) GetRevision() int64 {
	return t.Revision
}

func (t *RevisionBaseModel) SetRevision(revision int64) {
	t.Revision = revision
}

func (t *RevisionBaseModel) RevisionColumn() string {
	return "revision"
}

type DynamicSourceBaseModel struct {
	dbSource model.DbSource `gorm:"-"`
	dbType   model.DBType   `gorm:"-"`
}

func (t *DynamicSourceBaseModel) SetDbSource(source model.DbSource) {
	t.dbSource = source
}

func (t *DynamicSourceBaseModel) DbSource() model.DbSource {
	if t.dbSource == "" {
		return model.DbSource("main")
	}
	return t.dbSource
}

func (t *DynamicSourceBaseModel) SetDbType(typ model.DBType) {
	t.dbType = typ
}

func (t *DynamicSourceBaseModel) DbType() model.DBType {
	if t.dbType == "" {
		return model.DBTypeRelation
	}
	return t.dbType
}

type OwnerBaseModel struct {
	Owner string `json:"owner,omitempty" gorm:"column:owner"`
}

func (t *OwnerBaseModel) GetOwner() string {
	return t.Owner
}

type BaseModelOnlyDate struct {
	CreateBy string    `json:"create_by,omitempty" gorm:"column:create_by"`
	CreateAt time.Time `json:"create_at,omitempty" gorm:"column:create_at;autoCreateTime"`
	UpdateBy string    `json:"update_by,omitempty" gorm:"column:update_by"`
	UpdateAt time.Time `json:"update_at,omitempty" gorm:"column:update_at;autoUpdateTime"`
}

func (t *BaseModelOnlyDate) DbSource() model.DbSource {
	return model.DbSource("main")
}

func (t *BaseModelOnlyDate) DbType() model.DBType {
	return model.DBTypeRelation
}
func (t *BaseModelOnlyDate) Create() {
	t.CreateAt = clock.Now()
	t.UpdateAt = clock.Now()
}

func (t *BaseModelOnlyDate) Update() {
	t.UpdateAt = clock.Now()
}
