package dataaccess

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"gorm.io/gorm"

	"vane-common/pkg/dataaccess/cache"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/internal/db"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/safe"
)

func Accessor[T model.Table](dbName string) DataAccessor[T] {
	return DataAccessor[T]{}
}

type IDataAccessor[T model.Table] interface {
	Create(ctx context.Context, data T) error
	CreateWithTtl(ctx context.Context, data T, ttl time.Duration) error
	Get(ctx context.Context, data T) (T, error)
	GetByID(ctx context.Context, id field.IID) (T, error)
	First(ctx context.Context, conditions ...QueryCondition) (T, error)
	List(ctx context.Context, conditions ...QueryCondition) ([]T, error)
	Count(ctx context.Context, conditions ...QueryCondition) (int64, error)
	Save(ctx context.Context, data T, options updateOpt[T]) error
	Update(ctx context.Context, data T, options updateOpt[T]) error
	UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, options updateOpt[T]) error
	Delete(ctx context.Context, data T) error
	DeleteById(ctx context.Context, id field.IID) error
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
	Clean(ctx context.Context) error
	ListWithOption(ctx context.Context, results any, options ...ListOption) error
}

type updateOpt[T model.Table] struct {
	updateFieldLen int
	updateFields   []string
	ifRetry        func(ctx context.Context, newRecord T) (T, error)
}

func (o *updateOpt[T]) setFields(fields []string) {
	o.updateFields = fields
	o.updateFieldLen = len(o.updateFields)
}
func (o *updateOpt[T]) setIfRetry(ifRetry func(ctx context.Context, newRecord T) (T, error)) {
	o.ifRetry = ifRetry
}

type UpdateOptions interface {
	apply(options any)
}

type onlyFieldsOptions struct {
	fields []string
}

func (o *onlyFieldsOptions) apply(options any) {
	if opt, ok := options.(interface {
		setFields([]string)
	}); ok {
		opt.setFields(o.fields)
	}
}

func OnlyFields(fields ...string) UpdateOptions {
	return &onlyFieldsOptions{fields: fields}
}

// BuildUpdateOpt 将 UpdateOptions 转换为 updateOpt，供直接使用 KvDBDataAccessor 等实现时调用
func BuildUpdateOpt[T model.Table](opts ...UpdateOptions) updateOpt[T] {
	opt := updateOpt[T]{}
	for _, o := range opts {
		o.apply(&opt)
	}
	return opt
}

type ifRetryOptions[T model.Table] struct {
	ifRetry func(ctx context.Context, newRecord T) (T, error)
}

func (o *ifRetryOptions[T]) apply(options any) {
	if opt, ok := options.(interface {
		setIfRetry(func(ctx context.Context, newRecord T) (T, error))
	}); ok {
		opt.setIfRetry(o.ifRetry)
	}
}

func IfRetry[T model.Table](ifRetry func(ctx context.Context, newRecord T) (T, error)) UpdateOptions {
	return &ifRetryOptions[T]{ifRetry: ifRetry}
}

type DataAccessor[T model.Table] struct {
}

// WithModel creates a DynamicDataAccessor using the given model instance.
// This enables migration from DataAccessor to dynamic source routing without
// changing the accessor type at the call site.
func (d *DataAccessor[T]) WithModel(t T) DynamicDataAccessor[T] {
	return *DynamicAccessor[T](t)
}

func (d *DataAccessor[T]) Create(ctx context.Context, data T) error {
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	err = accessor.Create(ctx, data)
	if err != nil {
		return err
	}
	d.broadcast(data, ListenActionCreate)
	return nil
}
func (d *DataAccessor[T]) CreateWithTtl(ctx context.Context, data T, ttl time.Duration) error {
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	err = accessor.CreateWithTtl(ctx, data, ttl)
	if err != nil {
		return err
	}
	d.broadcast(data, ListenActionCreate)
	return nil
}

type cacheKey string

func (k cacheKey) Key(args ...any) string {
	return fmt.Sprintf(string(k), args...)
}

const (
	cacheKeyPrefixQuery cacheKey = "query/%s"
	cacheKeyPrefixGet   cacheKey = "query/%s/get/%s"
	cacheKeyPrefixFirst cacheKey = "query/%s/first/%s"
	cacheKeyPrefixList  cacheKey = "query/%s/list/%s"
	cacheKeyPrefixCount cacheKey = "query/%s/count/%s"
)

func (d *DataAccessor[T]) Get(ctx context.Context, data T) (T, error) {
	if ok, _ := IsCacheModel(data); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixGet.Key(data.TableName(), data.GetID().String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		var t T
		return t, err
	}
	result, err := accessor.Get(ctx, data)
	if err != nil {
		var t T
		return t, err
	}
	if ok, cacheTtl := IsCacheModel(data); ok {
		cache.SetCache(cacheKeyPrefixGet.Key(data.TableName(), data.GetID().String()), result, cacheTtl)
	}
	return result, nil
}
func (d *DataAccessor[T]) GetByID(ctx context.Context, id field.IID) (T, error) {
	var t T = newT[T]()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixGet.Key(t.TableName(), id.String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		var t T
		return t, err
	}
	result, err := accessor.GetByID(ctx, id)
	if err != nil {
		var t T
		return t, err
	}
	if ok, cacheTtl := IsCacheModel(t); ok {
		cache.SetCache(cacheKeyPrefixGet.Key(t.TableName(), id.String()), result, cacheTtl)
	}
	return result, nil
}
func (d *DataAccessor[T]) First(ctx context.Context, conditions ...QueryCondition) (T, error) {
	var t T = newT[T]()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixFirst.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		var t T
		return t, err
	}

	result, err := accessor.First(ctx, conditions...)
	if err != nil {
		var t T
		return t, err
	}
	if ok, cacheTtl := IsCacheModel(t); ok {
		cache.SetCache(cacheKeyPrefixFirst.Key(t.TableName(), QueryConditions(conditions).String()), result, cacheTtl)
	}
	return result, nil
}

func (d *DataAccessor[T]) List(ctx context.Context, conditions ...QueryCondition) ([]T, error) {
	var t T = newT[T]()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[[]T](cacheKeyPrefixList.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return nil, err
	}
	result, err := accessor.List(ctx, conditions...)
	if err != nil {
		return nil, err
	}
	if ok, cacheTtl := IsCacheModel(t); ok {
		cache.SetCache(cacheKeyPrefixList.Key(t.TableName(), QueryConditions(conditions).String()), result, cacheTtl)
	}
	return result, nil
}

func (d *DataAccessor[T]) Count(ctx context.Context, conditions ...QueryCondition) (int64, error) {
	var t T = newT[T]()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[int64](cacheKeyPrefixCount.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return 0, err
	}
	result, err := accessor.Count(ctx, conditions...)
	if err != nil {
		return 0, err
	}
	if ok, cacheTtl := IsCacheModel(t); ok {
		cache.SetCache(cacheKeyPrefixCount.Key(t.TableName(), QueryConditions(conditions).String()), result, cacheTtl)
	}
	return result, nil
}

func (d *DataAccessor[T]) ListWithOption(ctx context.Context, results any, options ...ListOption) error {
	accessor, err := GetDataAccessor[T](newT[T]().DbType())
	if err != nil {
		return err
	}
	err = accessor.ListWithOption(ctx, results, options...)
	if err != nil {
		return err
	}
	return nil
}

func (d *DataAccessor[T]) Save(ctx context.Context, data T, options ...UpdateOptions) error {
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	opt := updateOpt[T]{}
	for _, option := range options {
		option.apply(&opt)
	}
	// 必须要在save前判断
	isCreate := data.GetID().IsNil()
	err = accessor.Save(ctx, data, opt)
	if err != nil {
		return err
	}
	d.cleanCache(data)
	if isCreate {
		d.broadcast(data, ListenActionCreate)
	} else {
		d.broadcast(data, ListenActionUpdate)
	}
	return nil
}

func (d *DataAccessor[T]) Update(ctx context.Context, data T, options ...UpdateOptions) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	opt := updateOpt[T]{}
	for _, option := range options {
		option.apply(&opt)
	}
	err = accessor.Update(ctx, data, opt)
	if err != nil {
		return err
	}
	d.cleanCache(data)
	d.broadcast(data, ListenActionUpdate)
	return nil
}

func (d *DataAccessor[T]) UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, options ...UpdateOptions) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	opt := updateOpt[T]{}
	for _, option := range options {
		option.apply(&opt)
	}
	err = accessor.UpdateWithTtl(ctx, data, ttl, opt)
	if err != nil {
		return err
	}
	d.cleanCache(data)
	d.broadcast(data, ListenActionUpdate)
	return nil
}
func (d *DataAccessor[T]) Delete(ctx context.Context, data T) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	accessor, err := GetDataAccessor[T](data.DbType())
	if err != nil {
		return err
	}
	err = accessor.Delete(ctx, data)
	if err != nil {
		return err
	}
	d.cleanCache(data)
	d.broadcast(data, ListenActionDelete)
	return nil
}

func (d *DataAccessor[T]) DeleteById(ctx context.Context, id field.IID) error {
	if id.IsNil() {
		return ErrIDRequired
	}
	var t T = newT[T]()
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return err
	}
	err = accessor.DeleteById(ctx, id)
	if err != nil {
		return err
	}
	t.SetID(id)
	d.cleanCache(t)
	d.broadcast(t, ListenActionDelete)
	return nil
}

// 内部事务 一般用于内部事务
func (d *DataAccessor[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	var t T = newT[T]()
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return err
	}
	return accessor.Transaction(ctx, fn)
}

func (d *DataAccessor[T]) Clean(ctx context.Context) error {
	var t T = newT[T]()
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return err
	}
	return accessor.Clean(ctx)
}

func (d *DataAccessor[T]) cleanCache(data T) {
	if ok, _ := IsCacheModel(data); ok {
		cache.RemoveCacheWithPrefix(cacheKeyPrefixQuery.Key(data.TableName()))
	}
}
func (d *DataAccessor[T]) broadcast(data T, action ListenAction) {
	if listener, ok := GetDataListener(data); ok {
		safe.Go(func() { listener(ListenItem[model.Table]{Model: data, Action: action}) })
	}
}

// Transaction 通用事务 一般用于外部事务
func Transaction(ctx context.Context, dbType model.DBType, dbSource model.DbSource, fn func(ctx context.Context) error) error {
	switch dbType {
	case model.DBTypeRelation:
		tx, ok := db.GetWithTx(ctx, dbSource)
		if ok {
			return fn(ctx)
		}
		return tx.Transaction(func(tx *gorm.DB) error {
			return fn(db.StartTx(ctx, dbSource, tx))
		})
	case model.DBTypeKv:
		return getKVDB(dbSource).Transaction(ctx, func(ctx context.Context) error {
			return fn(getKVDB(dbSource).StartTx(ctx))
		})
	}
	return fmt.Errorf("db type %s not supported", dbType)
}

// 动态注册表：存储 dbType 到基础访问器工厂函数的映射
var (
	registeredAccessorFactories = make(map[model.DBType]func(constructor func() model.Table) IExtendDataAccessor)
)

// IExtendDataAccessor 基础访问器接口，不带泛型
type IExtendDataAccessor interface {
	Create(ctx context.Context, data model.Table) error
	CreateWithTtl(ctx context.Context, data model.Table, ttl time.Duration) error
	Get(ctx context.Context, data model.Table) (model.Table, error)
	GetByID(ctx context.Context, id field.IID) (model.Table, error)
	First(ctx context.Context, conditions ...QueryCondition) (model.Table, error)
	List(ctx context.Context, conditions ...QueryCondition) ([]model.Table, error)
	Count(ctx context.Context, conditions ...QueryCondition) (int64, error)
	Save(ctx context.Context, data model.Table, opt updateOpt[model.Table]) error
	Update(ctx context.Context, data model.Table, opt updateOpt[model.Table]) error
	UpdateWithTtl(ctx context.Context, data model.Table, ttl time.Duration, opt updateOpt[model.Table]) error
	Delete(ctx context.Context, data model.Table) error
	DeleteById(ctx context.Context, id field.IID) error
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
	Clean(ctx context.Context) error
	ListWithOption(ctx context.Context, results any, options ...ListOption) error
}

// GenericAccessorWrapper 包装器，将 IBaseDataAccessor 适配到 IDataAccessor[T]
type GenericAccessorWrapper[T model.Table] struct {
	baseAccessor IExtendDataAccessor
}

func (w *GenericAccessorWrapper[T]) ListWithOption(ctx context.Context, results any, options ...ListOption) error {
	return w.baseAccessor.ListWithOption(ctx, results, options...)
}

func (w *GenericAccessorWrapper[T]) Create(ctx context.Context, data T) error {
	return w.baseAccessor.Create(ctx, data)
}

func (w *GenericAccessorWrapper[T]) CreateWithTtl(ctx context.Context, data T, ttl time.Duration) error {
	return w.baseAccessor.CreateWithTtl(ctx, data, ttl)
}

func (w *GenericAccessorWrapper[T]) Get(ctx context.Context, data T) (T, error) {
	result, err := w.baseAccessor.Get(ctx, data)
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

func (w *GenericAccessorWrapper[T]) GetByID(ctx context.Context, id field.IID) (T, error) {
	var zero T
	result, err := w.baseAccessor.GetByID(ctx, id)
	if err != nil {
		return zero, err
	}
	return result.(T), nil
}

func (w *GenericAccessorWrapper[T]) First(ctx context.Context, conditions ...QueryCondition) (T, error) {
	var zero T
	result, err := w.baseAccessor.First(ctx, conditions...)
	if err != nil {
		return zero, err
	}
	return result.(T), nil
}

func (w *GenericAccessorWrapper[T]) List(ctx context.Context, conditions ...QueryCondition) ([]T, error) {
	results, err := w.baseAccessor.List(ctx, conditions...)
	if err != nil {
		return nil, err
	}
	// 转换 []model.Table 到 []T
	converted := make([]T, len(results))
	for i, r := range results {
		converted[i] = r.(T)
	}
	return converted, nil
}

func (w *GenericAccessorWrapper[T]) Count(ctx context.Context, conditions ...QueryCondition) (int64, error) {
	return w.baseAccessor.Count(ctx, conditions...)
}
func convertOpt[T model.Table](opt updateOpt[T]) updateOpt[model.Table] {
	var retry func(ctx context.Context, newRecord model.Table) (model.Table, error)

	if opt.ifRetry != nil {
		retry = func(ctx context.Context, newRecord model.Table) (model.Table, error) {
			return opt.ifRetry(ctx, newRecord.(T))
		}
	}

	return updateOpt[model.Table]{
		updateFields: opt.updateFields,
		ifRetry:      retry,
	}
}

func (w *GenericAccessorWrapper[T]) Save(ctx context.Context, data T, opt updateOpt[T]) error {
	return w.baseAccessor.Save(ctx, data, convertOpt(opt))
}

func (w *GenericAccessorWrapper[T]) Update(ctx context.Context, data T, opt updateOpt[T]) error {
	return w.baseAccessor.Update(ctx, data, convertOpt(opt))
}

func (w *GenericAccessorWrapper[T]) UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, opt updateOpt[T]) error {
	return w.baseAccessor.UpdateWithTtl(ctx, data, ttl, convertOpt(opt))
}

func (w *GenericAccessorWrapper[T]) Delete(ctx context.Context, data T) error {
	return w.baseAccessor.Delete(ctx, data)
}

func (w *GenericAccessorWrapper[T]) DeleteById(ctx context.Context, id field.IID) error {
	return w.baseAccessor.DeleteById(ctx, id)
}

func (w *GenericAccessorWrapper[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return w.baseAccessor.Transaction(ctx, fn)
}

func (w *GenericAccessorWrapper[T]) Clean(ctx context.Context) error {
	return w.baseAccessor.Clean(ctx)
}

// RegisterAccessorFactory 注册基础访问器工厂函数（不带泛型的实现）
// factory 函数接收一个 model.Table 实例，返回一个绑定了该 table 的访问器
func RegisterAccessorFactory(dbType model.DBType, factory func(constructor func() model.Table) IExtendDataAccessor) {
	registeredAccessorFactories[dbType] = factory
}

func GetDataAccessor[T model.Table](dbType model.DBType) (IDataAccessor[T], error) {
	switch dbType {
	case model.DBTypeRelation:
		return &GormDataAccessor[T]{}, nil
	case model.DBTypeKv:
		return &KvDBDataAccessor[T]{}, nil
	}
	if factory, ok := registeredAccessorFactories[dbType]; ok {
		baseAccessor := factory(func() model.Table {
			return newT[T]()
		})
		return &GenericAccessorWrapper[T]{
			baseAccessor: baseAccessor,
		}, nil
	}
	return nil, fmt.Errorf("db type %s not supported", dbType)
}

func DataAccessorFactory[T model.Table]() map[model.DBType]IDataAccessor[T] {
	result := map[model.DBType]IDataAccessor[T]{
		model.DBTypeRelation: &GormDataAccessor[T]{},
		model.DBTypeKv:       &KvDBDataAccessor[T]{},
	}

	// 添加所有动态注册的访问器
	for dbType, factory := range registeredAccessorFactories {
		baseAccessor := factory(func() model.Table {
			return newT[T]()
		})
		result[dbType] = &GenericAccessorWrapper[T]{
			baseAccessor: baseAccessor,
		}
	}

	return result
}
func getInt(value any) (int, bool) {
	switch t := value.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case int32:
		return int(t), true
	case int16:
		return int(t), true
	case int8:
		return int(t), true
	}
	return 0, false
}

func getFloat(value any) (float64, bool) {
	switch t := value.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case int16:
		return float64(t), true
	case int8:
		return float64(t), true
	}
	return 0, false
}
func newT[T model.Table]() T {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		t = reflect.New(elem(typ)).Interface().(T)
	}
	return t
}
func elem(p reflect.Type) reflect.Type {
	if p.Kind() == reflect.Pointer {
		return elem(p.Elem())
	}
	return p
}

var (
	cacheModels = make(map[string]time.Duration)
	cacheMutex  sync.RWMutex
)

func EnableCache(model model.Table, ttl time.Duration) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cacheModels[model.TableName()] = ttl
}

func IsCacheModel(model model.Table) (bool, time.Duration) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	ttl, ok := cacheModels[model.TableName()]
	if !ok {
		return false, 0
	}
	return true, ttl
}

type ListenAction string

const (
	ListenActionCreate ListenAction = "create"
	ListenActionUpdate ListenAction = "update"
	ListenActionDelete ListenAction = "delete"
)

type ListenItem[T model.Table] struct {
	Model  T
	Action ListenAction
}

var (
	listenerModels = make(map[string]func(item ListenItem[model.Table]))
	listenerMutex  sync.RWMutex
)

func RegisterDataListen(model model.Table, listener func(item ListenItem[model.Table])) {
	listenerMutex.Lock()
	defer listenerMutex.Unlock()
	if model != nil {
		listenerModels[model.TableName()] = listener
	}
}

func GetDataListener(model model.Table) (func(item ListenItem[model.Table]), bool) {
	if model == nil {
		return nil, false
	}
	listenerMutex.RLock()
	defer listenerMutex.RUnlock()
	listener, ok := listenerModels[model.TableName()]
	return listener, ok
}
