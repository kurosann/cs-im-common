package dataaccess

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"time"

	"vane-common/pkg/clock"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/internal/db"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var _ IDataAccessor[model.Table] = (*GormDataAccessor[model.Table])(nil)

type GormDataAccessor[T model.Table] struct {
	modelInstance T
	hasModel      bool
}

// modelTemplate 返回用于解析表名/数据源模板的模型实例。
//
// DynamicAccessor.WithModel 注入的实例优先（其 TableName() 可能按分片变化，
// 例如 im 日志的 im_message_{shard:02d}）；未注入时退回类型零值。
// 查询方法必须走这里——否则分表路由会退化成零值表名，静默读写错表。
func (d *GormDataAccessor[T]) modelTemplate() T {
	if d.hasModel {
		return d.modelInstance
	}
	return newT[T]()
}

func (d *GormDataAccessor[T]) source(t T) model.DbSource {
	if d.hasModel {
		return d.modelInstance.DbSource()
	}
	return t.DbSource()
}

func (d *GormDataAccessor[T]) Create(ctx context.Context, data T) error {
	data.Create()
	dbSource := d.source(data)
	return db.Get(ctx, dbSource).Table(data.TableName()).Create(data).Error
}

func (d *GormDataAccessor[T]) CreateWithTtl(ctx context.Context, data T, ttl time.Duration) error {
	if expireAtTable, ok := any(data).(model.ExpireAtTable); ok {
		setExpireAt(expireAtTable, ttl)
	}
	return d.Create(ctx, data)
}

func (d *GormDataAccessor[T]) Get(ctx context.Context, data T) (T, error) {
	return d.GetByID(ctx, data.GetID())
}

func (d *GormDataAccessor[T]) GetByID(ctx context.Context, id field.IID) (T, error) {
	t := d.modelTemplate()
	dbSource := d.source(t)
	tidColumn := t.IDColumn()
	if tidColumn == "" {
		logger.Panicf(ctx, "%s GetIDColumn() is empty", t.TableName())
	}
	db := db.Get(ctx, dbSource).Table(t.TableName())
	if expireAtTable, ok := any(t).(model.ExpireAtTable); ok {
		if column := expireAtTable.ExpireAtColumn(); column != "" {
			db = db.Where("("+column+" IS NULL OR "+column+" > ?)", time.Now())
		}
	}
	if err := db.Where(tidColumn+" = ?", id.Val()).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return t, fmt.Errorf("with id %v: %w", id.Val(), ErrDataNotFound)
		}
		return t, err
	}
	return t, nil
}
func (d *GormDataAccessor[T]) First(ctx context.Context, conditions ...QueryCondition) (T, error) {
	t := d.modelTemplate()
	conditions = append(conditions, Limit(1))
	list, err := d.List(ctx, conditions...)
	if err != nil {
		return t, err
	}
	if len(list) == 0 {
		return t, ErrDataNotFound
	}
	return list[0], nil
}

func (d *GormDataAccessor[T]) List(ctx context.Context, conditions ...QueryCondition) ([]T, error) {
	t := d.modelTemplate()
	var result []T

	dbSource := d.source(t)
	db := db.Get(ctx, dbSource).Table(t.TableName())

	db = getDbWithConditions(db, t.TableName(), nil, conditions...)

	if expireAtTable, ok := any(t).(model.ExpireAtTable); ok {
		if column := expireAtTable.ExpireAtColumn(); column != "" {
			db = db.Where("("+column+" IS NULL OR "+column+" > ?)", time.Now())
		}
	}

	return result, db.Find(&result).Error
}

func (d *GormDataAccessor[T]) Count(ctx context.Context, conditions ...QueryCondition) (int64, error) {
	t := d.modelTemplate()
	var result int64
	dbSource := d.source(t)
	db := db.Get(ctx, dbSource).Table(t.TableName())

	db = getDbWithConditions(db, t.TableName(), nil, conditions...)
	if expireAtTable, ok := any(t).(model.ExpireAtTable); ok {
		if column := expireAtTable.ExpireAtColumn(); column != "" {
			db = db.Where("("+column+" IS NULL OR "+column+" > ?)", time.Now())
		}
	}

	return result, db.Count(&result).Error
}

func (d *GormDataAccessor[T]) Save(ctx context.Context, data T, opt updateOpt[T]) error {
	if data.GetID().IsNil() {
		return d.Create(ctx, data)
	} else {
		if _, err := d.Get(ctx, data); err != nil {
			if errors.Is(err, ErrDataNotFound) {
				return d.Create(ctx, data)
			}
			return err
		}
		return d.update(ctx, data, opt)
	}
}

func (d *GormDataAccessor[T]) Update(ctx context.Context, data T, opt updateOpt[T]) error {
	return d.update(ctx, data, opt)
}

func (d *GormDataAccessor[T]) update(ctx context.Context, data T, opt updateOpt[T]) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}

	data.Update()
	db := d.buildUpdate(ctx, data, opt)
	tx := db.Updates(data)
	if tx.Error != nil {
		return tx.Error
	}

	if _, ok := any(data).(model.RevisionTable); ok {
		if tx.RowsAffected == 0 && opt.ifRetry != nil {
			for {
				select {
				case <-ctx.Done():
					return ErrRevisionMismatch
				default:
					newRecord, err := d.GetByID(ctx, data.GetID())
					if err != nil {
						return err
					}
					newRecord, err = opt.ifRetry(ctx, newRecord)
					if err != nil {
						return err
					}
					db = d.buildUpdate(ctx, newRecord, opt)
					tx = db.Updates(newRecord)
					if tx.Error != nil {
						return tx.Error
					}
					if tx.RowsAffected != 0 {
						return nil
					}
				}
			}
		}
	}
	return nil
}
func (d *GormDataAccessor[T]) buildUpdate(ctx context.Context, data T, opt updateOpt[T]) *gorm.DB {
	dbSource := d.source(data)
	tidColumn := data.IDColumn()
	if tidColumn == "" {
		logger.Panicf(ctx, "%s GetIDColumn() is empty", data.TableName())
	}

	db := db.Get(ctx, dbSource).
		Table(data.TableName()).
		Where(tidColumn+" = ?", data.GetID().Val())

	// revision
	if revisionTable, ok := any(data).(model.RevisionTable); ok {
		oldRevision := revisionTable.GetRevision()

		revisionColumn := revisionTable.RevisionColumn()
		db = db.Where(
			revisionColumn+" = ?",
			oldRevision,
		)

		// revision +1
		if oldRevision >= math.MaxInt64-1 {
			revisionTable.SetRevision(0)
		} else {
			revisionTable.SetRevision(oldRevision + 1)
		}
		if opt.updateFieldLen > 0 {
			opt.updateFields = append(opt.updateFields, revisionColumn)
		}
	}

	if opt.updateFieldLen > 0 {
		db = db.Select(opt.updateFields)
	} else {
		db = db.Select("*")
	}
	return db
}
func (d *GormDataAccessor[T]) UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, opt updateOpt[T]) error {
	if expireAtTable, ok := any(data).(model.ExpireAtTable); ok {
		setExpireAt(expireAtTable, ttl)
		if column := expireAtTable.ExpireAtColumn(); column != "" && len(opt.updateFields) > 0 && !slices.Contains(opt.updateFields, column) {
			if opt.updateFieldLen > 0 {
				opt.updateFields = append(opt.updateFields, column)
			}
		}
	}
	return d.Update(ctx, data, opt)
}

func setExpireAt(expireAtTable model.ExpireAtTable, ttl time.Duration) {
	if ttl <= 0 {
		expireAtTable.SetExpireAt(nil)
		return
	}
	expireAt := time.Now().Add(ttl)
	expireAtTable.SetExpireAt(&expireAt)
}
func (d *GormDataAccessor[T]) Delete(ctx context.Context, data T) error {
	return d.DeleteById(ctx, data.GetID())
}

func (d *GormDataAccessor[T]) DeleteById(ctx context.Context, id field.IID) error {
	if id.IsNil() {
		return ErrIDRequired
	}
	t := d.modelTemplate()
	dbSource := d.source(t)
	tidColumn := t.IDColumn()
	if tidColumn == "" {
		logger.Panicf(ctx, "%s GetIDColumn() is empty", t.TableName())
	}
	return db.Get(ctx, dbSource).Table(t.TableName()).Delete(t, tidColumn+" = ?", id.Val()).Error
}

// 内部事务 一般用于内部事务
func (d *GormDataAccessor[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	t := d.modelTemplate()
	dbSource := d.source(t)
	tx, ok := db.GetWithTx(ctx, dbSource)
	if ok {
		return fn(ctx)
	}
	return tx.Transaction(func(tx *gorm.DB) error {
		return fn(db.StartTx(ctx, dbSource, tx))
	})
}

func (d *GormDataAccessor[T]) Clean(ctx context.Context) error {
	t := d.modelTemplate()
	dbSource := d.source(t)
	if expireAtTable, ok := any(t).(model.ExpireAtTable); ok {
		column := expireAtTable.ExpireAtColumn()
		if column == "" {
			logger.Panicf(ctx, "%s ExpireAtColumn() is empty", t.TableName())
		}
		return db.Get(ctx, dbSource).Table(t.TableName()).Where("("+column+" IS NOT NULL AND "+column+" < ?)", clock.Now()).Delete(t).Error
	}
	return nil
}
func getDbWithConditions(db *gorm.DB, table string, joinTables []string, conditions ...QueryCondition) *gorm.DB {
	for _, condition := range conditions {
		// 解析字段名：优先使用FieldFunc，否则使用Field字符串
		fieldName := condition.Field
		if condition.FieldFunc != nil {
			fieldName = condition.FieldFunc(table, joinTables...)
		} else if condition.Field != "" {
			fieldName = fmt.Sprintf("%s.%s", table, fieldName)
		}

		switch condition.Type {
		case QueryConditionTypeDistinct:
			if fields, ok := getSliceString(condition.Value); ok {
				db = db.Distinct(strings.Join(fields, ", "))
			}
		case QueryConditionTypeOrder:
			fieldName := condition.Field
			if condition.FieldFunc != nil {
				fieldName = condition.FieldFunc(table, joinTables...)
			} else if condition.Field != "" {
				fieldName = fmt.Sprintf("%s.`%s`", table, fieldName)
			}
			db = db.Order(fmt.Sprintf("%s %s", fieldName, condition.Value))
		case QueryConditionTypeLimit:
			if limit, ok := getInt(condition.Value); ok {
				db = db.Limit(limit)
			}
		case QueryConditionTypeOffset:
			if offset, ok := getInt(condition.Value); ok {
				db = db.Offset(offset)
			}
		case QueryConditionTypeEq:
			db = db.Where(fieldName+" = ?", condition.Value)
		case QueryConditionTypeNe:
			db = db.Where(fieldName+" <> ?", condition.Value)
		case QueryConditionTypeGt:
			db = db.Where(fieldName+" > ?", condition.Value)
		case QueryConditionTypeGte:
			db = db.Where(fieldName+" >= ?", condition.Value)
		case QueryConditionTypeLt:
			db = db.Where(fieldName+" < ?", condition.Value)
		case QueryConditionTypeLte:
			db = db.Where(fieldName+" <= ?", condition.Value)
		case QueryConditionTypeIn:
			db = db.Where(fieldName+" IN (?)", condition.Value)
		case QueryConditionTypeNotIn:
			db = db.Where(fieldName+" NOT IN (?)", condition.Value)
		case QueryConditionTypeLike:
			db = db.Where(fieldName+" LIKE ?", fmt.Sprintf("%%%v%%", condition.Value))
		case QueryConditionTypeNotLike:
			db = db.Where(fieldName+" NOT LIKE ?", fmt.Sprintf("%%%v%%", condition.Value))
		case QueryConditionTypeIsNull:
			db = db.Where(fieldName + " IS NULL")
		case QueryConditionTypeIsNotNull:
			db = db.Where(fieldName + " IS NOT NULL")
		case QueryConditionTypeBetween:
			if s, ok := getSlice(condition.Value); ok && len(s) >= 2 {
				db = db.Where(fieldName+" BETWEEN ? AND ?", s[0], s[1])
			}
		case QueryConditionTypeNotBetween:
			if s, ok := getSlice(condition.Value); ok && len(s) >= 2 {
				db = db.Where(fieldName+" NOT BETWEEN ? AND ?", s[0], s[1])
			}
		case QueryConditionTypeAnd:
			if sub, ok := getConditions(condition.Value); ok && len(sub) > 0 {
				subDb := getDbWithConditions(db.Session(&gorm.Session{NewDB: true}), table, joinTables, sub...)
				db = db.Where(subDb)
			}
		case QueryConditionTypeOr:
			if sub, ok := getConditions(condition.Value); ok && len(sub) > 0 {
				subDb := buildOrSubQuery(db.Session(&gorm.Session{NewDB: true}), table, joinTables, sub)
				db = db.Where(subDb)
			}
		case QueryConditionTypeSkip:
			continue
		default:
			logger.Warn(db.Statement.Context, "unknown query condition type", zap.String("type", string(condition.Type)))
		}
	}
	return db
}

func getSlice(value any) ([]any, bool) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	res := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		res[i] = rv.Index(i).Interface()
	}
	return res, true
}

func getConditions(value any) ([]QueryCondition, bool) {
	conditions, ok := value.([]QueryCondition)
	return conditions, ok
}

// buildOrSubQuery 构建子查询，内部多个条件用 OR 连接
// 例如 Or(Eq("name","a"), Eq("name","b")) => WHERE name='a' OR name='b'
func buildOrSubQuery(db *gorm.DB, table string, joinTables []string, conditions []QueryCondition) *gorm.DB {
	for i, condition := range conditions {
		// 将每个条件包装成单个元素的 And 或直接条件，用 Or 连接
		subDb := getDbWithConditions(db.Session(&gorm.Session{NewDB: true}), table, joinTables, condition)
		if i == 0 {
			db = db.Where(subDb)
		} else {
			db = db.Or(subDb)
		}
	}
	return db
}

type SelectFunc func(table string, joinTables ...string) string

// ListConfig - ListWithOption 配置
type ListConfig struct {
	Conditions   []QueryCondition
	Joins        []JoinSpec
	SelectFields []SelectFunc
	OrderBy      SelectFunc
}

// JoinSpec - JOIN 配置
type JoinSpec struct {
	Table model.Table
	Type  JoinType
	On    *joinOn
}

// ListOption - Option 函数类型
type ListOption func(*ListConfig)

// JoinOption - JOIN 的 Option 函数类型
type JoinOption func(*JoinSpec)

// WithJoinType - 设置 JOIN 类型
func WithJoinType(t JoinType) JoinOption {
	return func(spec *JoinSpec) {
		spec.Type = t
	}
}

// WithJoin - 生成 JOIN Option
func WithJoin[J model.Table](on *joinOn, opts ...JoinOption) ListOption {
	return func(config *ListConfig) {
		spec := &JoinSpec{
			Table: newT[J](),
			Type:  JoinTypeLeft,
			On:    on,
		}
		for _, opt := range opts {
			opt(spec)
		}
		config.Joins = append(config.Joins, *spec)
	}
}

// WithSelect - 设置查询字段，每个函数接收主表名和JOIN表名生成一个字段
func WithSelect(f ...SelectFunc) ListOption {
	return func(config *ListConfig) {
		config.SelectFields = f
	}
}

// WithOrder - 设置排序
func WithOrder(f SelectFunc) ListOption {
	return func(config *ListConfig) {
		config.OrderBy = f
	}
}

// WithConditions - 设置查询条件
func WithConditions(conditions ...QueryCondition) ListOption {
	return func(config *ListConfig) {
		config.Conditions = append(config.Conditions, conditions...)
	}
}

// ListWithOption - 支持 复杂查询 的 List 查询
// results 用于接收 JOIN 的关联数据（仅 KV 实现有意义，GORM 的 JOIN 数据在主结果中）
func (d *GormDataAccessor[T]) ListWithOption(ctx context.Context, results any, options ...ListOption) error {
	t := d.modelTemplate()

	dbSource := d.source(t)
	db := db.Get(ctx, dbSource).Table(t.TableName())

	// 构建 config
	config := &ListConfig{}
	for _, opt := range options {
		opt(config)
	}

	// 处理 JOIN
	for _, join := range config.Joins {
		db = db.Joins(fmt.Sprintf("%s JOIN %s ON %s.%s = %s.%s",
			join.Type, join.Table.TableName(), t.TableName(), join.On.Table1Field, join.Table.TableName(), join.On.Table2Field))
	}

	// 处理 ExpireAtTable - 过滤已过期的记录
	if expireAtTable, ok := any(t).(model.ExpireAtTable); ok {
		if column := expireAtTable.ExpireAtColumn(); column != "" {
			db = db.Where("("+t.TableName()+"."+column+" IS NULL OR "+t.TableName()+"."+column+" > ?)", time.Now())
		}
	}

	// 获取JOIN的表名列表
	joinTables := make([]string, len(config.Joins))
	for i, join := range config.Joins {
		joinTables[i] = join.Table.TableName()
	}

	// 处理 Conditions
	if len(config.Conditions) > 0 {
		db = getDbWithConditions(db, t.TableName(), joinTables, config.Conditions...)
	}

	// 处理 SelectFields
	if len(config.SelectFields) > 0 {
		fields := make([]string, len(config.SelectFields))
		for i, f := range config.SelectFields {
			fields[i] = f(t.TableName(), joinTables...)
		}
		db = db.Select(fields)
	}

	// 处理 OrderBy
	if config.OrderBy != nil {
		db = db.Order(config.OrderBy(t.TableName(), joinTables...))
	}

	return db.Find(results).Error
}
