package dataaccess

import (
	"context"
	"time"

	"vane-common/pkg/dataaccess/cache"
	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/safe"
)

// DynamicAccessor creates a DynamicDataAccessor with the specified dbSource.
// The dbSource is propagated to the underlying GormDataAccessor for query methods,
// enabling dynamic database source routing.
func DynamicAccessor[T model.Table](t T) *DynamicDataAccessor[T] {
	return &DynamicDataAccessor[T]{modelInstance: t, hasModel: true}
}

// DynamicDataAccessor is a data accessor that supports dynamic database source routing.
// Unlike DataAccessor which always uses the default source from the model, DynamicDataAccessor
// propagates its dbSource to the underlying accessor, enabling multi-database scenarios.
type DynamicDataAccessor[T model.Table] struct {
	modelInstance T
	hasModel      bool
}

// WithModel sets a model instance to use as a template for DbType() and TableName().
// When set (non-nil), this instance is used in query methods (GetByID, List, etc.)
// instead of creating a new one via newT[T](). This is useful when the model has
// pre-configured DbSource or DbType values.
// Returns itself for method chaining.
func (d *DynamicDataAccessor[T]) WithModel(t T) DynamicDataAccessor[T] {
	if d == nil {
		return DynamicDataAccessor[T]{}
	}
	var zero T
	if any(t) == any(zero) {
		return *d
	}
	d.modelInstance = t
	d.hasModel = true
	return *d
}

func (d *DynamicDataAccessor[T]) getModel() T {
	if d.hasModel {
		return d.modelInstance
	}
	return newT[T]()
}

func (d *DynamicDataAccessor[T]) getAccessor() (IDataAccessor[T], error) {
	t := d.getModel()
	accessor, err := GetDataAccessor[T](t.DbType())
	if err != nil {
		return nil, err
	}
	if ga, ok := accessor.(*GormDataAccessor[T]); ok {
		ga.modelInstance = d.modelInstance
		ga.hasModel = true
	}
	return accessor, nil
}

// applySource 将 modelInstance 的 DbSource/DbType 注入到查询返回的数据中，
// 确保后续 Save/Update/Delete 操作路由到正确的数据库。
func (d *DynamicDataAccessor[T]) applySource(data T) {
	if !d.hasModel {
		return
	}
	if ds, ok := any(data).(model.DynamicSourceTable); ok {
		ds.SetDbSource(d.modelInstance.DbSource())
		ds.SetDbType(d.modelInstance.DbType())
	}
}

func (d *DynamicDataAccessor[T]) Create(ctx context.Context, data T) error {
	d.applySource(data)
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) CreateWithTtl(ctx context.Context, data T, ttl time.Duration) error {
	d.applySource(data)
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) Get(ctx context.Context, data T) (T, error) {
	if ok, _ := IsCacheModel(data); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixGet.Key(data.TableName(), data.GetID().String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := d.getAccessor()
	if err != nil {
		var t T
		return t, err
	}
	result, err := accessor.GetByID(ctx, data.GetID())
	if err != nil {
		var t T
		return t, err
	}
	if ok, cacheTtl := IsCacheModel(data); ok {
		cache.SetCache(cacheKeyPrefixGet.Key(data.TableName(), data.GetID().String()), result, cacheTtl)
	}
	return result, nil
}

func (d *DynamicDataAccessor[T]) GetByID(ctx context.Context, id field.IID) (T, error) {
	t := d.getModel()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixGet.Key(t.TableName(), id.String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) First(ctx context.Context, conditions ...QueryCondition) (T, error) {
	t := d.getModel()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[T](cacheKeyPrefixFirst.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) List(ctx context.Context, conditions ...QueryCondition) ([]T, error) {
	t := d.getModel()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[[]T](cacheKeyPrefixList.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) Count(ctx context.Context, conditions ...QueryCondition) (int64, error) {
	t := d.getModel()
	if ok, _ := IsCacheModel(t); ok {
		value, ok := cache.GetCache[int64](cacheKeyPrefixCount.Key(t.TableName(), QueryConditions(conditions).String()))
		if ok {
			return value, nil
		}
	}
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) ListWithOption(ctx context.Context, results any, options ...ListOption) error {
	accessor, err := d.getAccessor()
	if err != nil {
		return err
	}
	err = accessor.ListWithOption(ctx, results, options...)
	if err != nil {
		return err
	}
	return nil
}

func (d *DynamicDataAccessor[T]) Save(ctx context.Context, data T, options ...UpdateOptions) error {
	d.applySource(data)
	accessor, err := d.getAccessor()
	if err != nil {
		return err
	}
	opt := updateOpt[T]{}
	for _, option := range options {
		option.apply(&opt)
	}
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

func (d *DynamicDataAccessor[T]) Update(ctx context.Context, data T, options ...UpdateOptions) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	d.applySource(data)
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, options ...UpdateOptions) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	d.applySource(data)
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) Delete(ctx context.Context, data T) error {
	if data.GetID().IsNil() {
		return ErrIDRequired
	}
	d.applySource(data)
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) DeleteById(ctx context.Context, id field.IID) error {
	if id.IsNil() {
		return ErrIDRequired
	}
	t := d.getModel()
	accessor, err := d.getAccessor()
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

func (d *DynamicDataAccessor[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	accessor, err := d.getAccessor()
	if err != nil {
		return err
	}
	return accessor.Transaction(ctx, fn)
}

func (d *DynamicDataAccessor[T]) Clean(ctx context.Context) error {
	accessor, err := d.getAccessor()
	if err != nil {
		return err
	}
	return accessor.Clean(ctx)
}

func (d *DynamicDataAccessor[T]) cleanCache(data T) {
	if ok, _ := IsCacheModel(data); ok {
		cache.RemoveCacheWithPrefix(cacheKeyPrefixQuery.Key(data.TableName()))
	}
}

func (d *DynamicDataAccessor[T]) broadcast(data T, action ListenAction) {
	if listener, ok := GetDataListener(data); ok {
		safe.Go(func() { listener(ListenItem[model.Table]{Model: data, Action: action}) })
	}
}
