package dataaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"vane-common/pkg/dataaccess/field"
	"vane-common/pkg/dataaccess/internal/kv"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/logger"

	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var _ IDataAccessor[model.Table] = (*KvDBDataAccessor[model.Table])(nil)

func getKVDB(source model.DbSource) store.IKVStore {
	return kv.Get(source).(store.IKVStore)
}

type KvDBDataAccessor[T model.Table] struct {
}

func (d *KvDBDataAccessor[T]) Create(ctx context.Context, data T) (err error) {
	return d.CreateWithTtl(ctx, data, 0)
}
func (d *KvDBDataAccessor[T]) CreateWithTtl(ctx context.Context, data T, ttl time.Duration) (err error) {
	data.Create()
	bs, err := marshalData(data)
	if err != nil {
		return err
	}
	return getKVDB(data.DbSource()).PutWithTtl(ctx, d.genKey(data.TableName(), data.GetID().String()), bs, ttl)
}
func (d *KvDBDataAccessor[T]) genKey(tablename string, id string) []byte {
	key := []byte(tablename)
	if id == "" {
		return key
	}
	return fmt.Appendf(key, ":%s", id)
}
func marshalData(data any) ([]byte, error) {
	if marshaler, ok := data.(model.TableMarshaler); ok {
		return marshaler.Marshal()
	}
	return json.Marshal(data)
}
func unmarshalData(data any, bs []byte) error {
	if marshaler, ok := data.(model.TableMarshaler); ok {
		return marshaler.Unmarshal(bs)
	}
	return json.Unmarshal(bs, data)
}

func (d *KvDBDataAccessor[T]) Get(ctx context.Context, data T) (T, error) {
	return d.GetByID(ctx, data.GetID())
}

func (d *KvDBDataAccessor[T]) GetByID(ctx context.Context, id field.IID) (T, error) {
	var t T = newT[T]()
	kvdb := getKVDB(t.DbSource())
	bs, err := kvdb.Get(ctx, d.genKey(t.TableName(), id.String()))
	if err != nil {
		if errors.Is(err, store.ErrDataNotFound) ||
			func() bool {
				classifier, ok := kvdb.(store.NotFoundClassifier)
				return ok && classifier.IsNotFound(err)
			}() {
			return t, fmt.Errorf("with id %v: %w", id.Val(), ErrDataNotFound)
		}
		return t, err
	}
	err = unmarshalData(t, bs)
	if err != nil {
		return t, err
	}
	return t, nil
}

func (d *KvDBDataAccessor[T]) First(ctx context.Context, conditions ...QueryCondition) (T, error) {
	list, err := d.List(ctx, conditions...)
	if err != nil {
		var t T = newT[T]()
		return t, err
	}
	if len(list) == 0 {
		var t T = newT[T]()
		return t, ErrDataNotFound
	}
	return list[0], nil
}

func (d *KvDBDataAccessor[T]) List(ctx context.Context, conditions ...QueryCondition) ([]T, error) {
	var t T = newT[T]()
	kv, err := getKVDB(t.DbSource()).Scan(ctx, []byte(t.TableName()), func(k string, v []byte) bool {
		return filterConditions(ctx, t, v, conditions...)
	})
	if err != nil {
		return nil, err
	}

	// 提取 distinct 字段
	distinctFields := getDistinctFields(conditions)

	var result []T
	seen := make(map[string]struct{})
	for _, value := range kv {
		var t T = newT[T]()
		err = unmarshalData(t, value)
		if err != nil {
			continue
		}
		// 去重：按指定字段的值生成 key，跳过重复
		if len(distinctFields) > 0 {
			key := buildDistinctKey(t, distinctFields)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		result = append(result, t)
	}
	return result, nil
}

func (d *KvDBDataAccessor[T]) Count(ctx context.Context, conditions ...QueryCondition) (int64, error) {
	var t T = newT[T]()
	return getKVDB(t.DbSource()).Count(ctx, []byte(t.TableName()), func(k string, v []byte) bool {
		return filterConditions(ctx, t, v, conditions...)
	})
}

func (d *KvDBDataAccessor[T]) Save(ctx context.Context, data T, opt updateOpt[T]) error {
	return d.Create(ctx, data)
}

func (d *KvDBDataAccessor[T]) UpdateWithTtl(ctx context.Context, data T, ttl time.Duration, opt updateOpt[T]) error {
	if len(opt.updateFields) > 0 {
		record, err := d.GetByID(ctx, data.GetID())
		if err != nil {
			return err
		}
		if err = applyUpdateFields(record, data, opt.updateFields); err != nil {
			return err
		}
		resultBs, err := marshalData(record)
		if err != nil {
			return err
		}
		return getKVDB(data.DbSource()).PutWithTtl(ctx, d.genKey(data.TableName(), data.GetID().String()), resultBs, ttl)
	}
	bs, err := marshalData(data)
	if err != nil {
		return err
	}
	return getKVDB(data.DbSource()).PutWithTtl(ctx, d.genKey(data.TableName(), data.GetID().String()), bs, ttl)
}

func (d *KvDBDataAccessor[T]) Update(ctx context.Context, data T, opt updateOpt[T]) error {
	return d.UpdateWithTtl(ctx, data, 0, opt)
}

func applyUpdateFields(dst any, src any, fields []string) error {
	for _, field := range fields {
		dstField, ok := findUpdateField(reflect.ValueOf(dst), field, true)
		if !ok {
			return fmt.Errorf("kv update field %q not found", field)
		}
		srcField, ok := findUpdateField(reflect.ValueOf(src), field, false)
		if !ok {
			return fmt.Errorf("kv update field %q not found in source", field)
		}
		if !srcField.Type().AssignableTo(dstField.Type()) {
			if !srcField.Type().ConvertibleTo(dstField.Type()) {
				return fmt.Errorf("kv update field %q type mismatch: %s cannot assign to %s", field, srcField.Type(), dstField.Type())
			}
			srcField = srcField.Convert(dstField.Type())
		}
		dstField.Set(srcField)
	}
	return nil
}

func findUpdateField(v reflect.Value, name string, requireSet bool) (reflect.Value, bool) {
	v = derefValue(v)
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "`")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := typ.Field(i)
		fv := v.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
			continue
		}
		if isUpdateFieldMatch(sf, name) {
			if !requireSet || fv.CanSet() {
				return fv, true
			}
			return reflect.Value{}, false
		}
		if sf.Anonymous {
			if fv.Kind() == reflect.Pointer && fv.IsNil() {
				if !requireSet || !fv.CanSet() || fv.Type().Elem().Kind() != reflect.Struct {
					continue
				}
				fv.Set(reflect.New(fv.Type().Elem()))
			}
			if field, ok := findUpdateField(fv, name, requireSet); ok {
				return field, true
			}
		}
	}
	return reflect.Value{}, false
}

func derefValue(v reflect.Value) reflect.Value {
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func isUpdateFieldMatch(field reflect.StructField, name string) bool {
	if name == "" {
		return false
	}
	if field.Name == name || strings.EqualFold(field.Name, name) {
		return true
	}
	if tagName := tagHead(field.Tag.Get("json")); tagName == name {
		return true
	}
	return gormColumn(field.Tag.Get("gorm")) == name
}

func tagHead(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

func gormColumn(tag string) string {
	for _, part := range strings.Split(tag, ";") {
		if strings.HasPrefix(part, "column:") {
			return strings.TrimPrefix(part, "column:")
		}
	}
	return ""
}

func (d *KvDBDataAccessor[T]) Delete(ctx context.Context, data T) error {
	return d.DeleteById(ctx, data.GetID())
}

func (d *KvDBDataAccessor[T]) DeleteById(ctx context.Context, id field.IID) error {
	var t T = newT[T]()
	return getKVDB(t.DbSource()).Del(ctx, d.genKey(t.TableName(), id.String()))
}

func (d *KvDBDataAccessor[T]) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	var t T = newT[T]()
	return getKVDB(t.DbSource()).Transaction(ctx, fn)
}

func (d *KvDBDataAccessor[T]) Clean(ctx context.Context) error {
	var t T = newT[T]()
	return getKVDB(t.DbSource()).Clean(ctx)
}

// ListWithOption - KV存储的List查询，支持Conditions和内存JOIN
// result 用于接收查询结果（主表数据 + JOIN 关联数据组合成 JSON 后反序列化）
func (d *KvDBDataAccessor[T]) ListWithOption(ctx context.Context, result any, options ...ListOption) error {
	var t T = newT[T]()

	// 构建 config
	config := &ListConfig{}
	for _, opt := range options {
		opt(config)
	}

	// 获取JOIN表名列表
	joinTables := make([]string, len(config.Joins))
	for i, join := range config.Joins {
		joinTables[i] = join.Table.TableName()
	}

	// 解析 Conditions 中的 FieldFunc
	var resolvedConditions []QueryCondition
	if len(config.Conditions) > 0 {
		resolvedConditions = d.resolveConditions(t.TableName(), joinTables, config.Conditions)
	}

	// 直接扫描主表，自己实现过滤逻辑
	kv, err := getKVDB(t.DbSource()).Scan(ctx, []byte(t.TableName()), func(k string, v []byte) bool {
		return filterConditions(ctx, t, v, resolvedConditions...)
	})
	if err != nil {
		return err
	}

	// 构建结果数据
	var combinedResults []map[string]any
	for _, v := range kv {
		var item T
		if marshaler, ok := any(&item).(model.TableMarshaler); ok {
			if err := marshaler.Unmarshal(v); err != nil {
				continue
			}
		} else {
			if err := json.Unmarshal(v, &item); err != nil {
				continue
			}
		}
		recordMap := d.extractFields(item)
		combined := make(map[string]any)
		for k, val := range recordMap {
			combined[k] = val
		}

		// 处理 JOIN
		for _, join := range config.Joins {
			onValue := recordMap[join.On.Table1Field]
			joinCondition := d.buildJoinCondition(join.On.Table2Field, onValue)
			joinRecords, _ := d.queryJoinedTable(ctx, join.Table, joinCondition)

			// 收集 join 结果为 JSON
			if len(joinRecords) > 0 {
				var joinValues []any
				for _, jr := range joinRecords {
					jrMap := d.extractFields(jr)
					joinValues = append(joinValues, jrMap)
				}
				combined[join.On.Table1Field+"s"] = joinValues // 简单命名，实际可调整
			} else if join.Type != JoinTypeInner {
				combined[join.On.Table1Field+"s"] = []any{}
			}
		}
		combinedResults = append(combinedResults, combined)
	}

	// 序列化为 JSON 后反序列化到 result
	if len(combinedResults) > 0 {
		bs, err := json.Marshal(combinedResults)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bs, result); err != nil {
			return err
		}
	}

	return nil
}

// extractFields - 从记录中提取字段值
func (d *KvDBDataAccessor[T]) extractFields(data any) map[string]any {
	var m map[string]any
	if marshaler, ok := data.(model.TableMarshaler); ok {
		bs, _ := marshaler.Marshal()
		json.Unmarshal(bs, &m)
	} else {
		bs, _ := json.Marshal(data)
		json.Unmarshal(bs, &m)
	}
	return m
}

// queryJoinedTable - 查询关联表
func (d *KvDBDataAccessor[T]) queryJoinedTable(ctx context.Context, table model.Table, condition QueryCondition) ([]model.Table, error) {
	// 使用反射调用对应类型的 List 方法
	typ := reflect.TypeOf(table)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// 创建新的表实例
	newTable := reflect.New(typ).Interface().(model.Table)
	tableName := newTable.TableName()

	kv, err := getKVDB(newTable.DbSource()).Scan(ctx, []byte(tableName), func(k string, v []byte) bool {
		return filterConditionsWithMap(ctx, table, v, condition)
	})
	if err != nil {
		return nil, err
	}

	var results []model.Table
	for _, value := range kv {
		var t model.Table = reflect.New(typ).Interface().(model.Table)
		if marshaler, ok := t.(model.TableMarshaler); ok {
			marshaler.Unmarshal(value)
		} else {
			json.Unmarshal(value, t)
		}
		results = append(results, t)
	}
	return results, nil
}

// buildJoinCondition - 根据字段值类型构建查询条件
func (d *KvDBDataAccessor[T]) buildJoinCondition(field string, value any) QueryCondition {
	switch v := value.(type) {
	case int:
		return Eq(field, v)
	case int64:
		return Eq(field, v)
	case float64:
		return Eq(field, v)
	case string:
		return Eq(field, v)
	default:
		return Eq(field, fmt.Sprintf("%v", v))
	}
}

// resolveConditions - 将QueryCondition中的FieldFunc解析为实际字段名
func (d *KvDBDataAccessor[T]) resolveConditions(table string, joinTables []string, conditions []QueryCondition) []QueryCondition {
	result := make([]QueryCondition, len(conditions))
	for i, c := range conditions {
		result[i] = c
		if c.FieldFunc != nil {
			result[i] = QueryCondition{
				Type:  c.Type,
				Field: c.FieldFunc(table, joinTables...),
				Value: c.Value,
			}
		}
	}
	return result
}

func filterConditions(ctx context.Context, obj any, value []byte, conditions ...QueryCondition) bool {
	if len(conditions) == 0 {
		return true // 没有条件，不过滤
	}

	// 使用 TableMarshaler 的 UnmarshalMap 或者 gjson
	if marshaler, ok := obj.(model.TableMarshaler); ok || len(conditions) > 10 {
		return filterConditionsWithMap(ctx, marshaler, value, conditions...)
	} else {
		return filterConditionsWithGjson(ctx, value, conditions...)
	}
}

func filterConditionsWithMap(ctx context.Context, marshaler any, value []byte, conditions ...QueryCondition) bool {
	// 转换成map然后根据字段匹配condition
	var m map[string]any
	if marshaler, ok := marshaler.(model.TableMarshaler); ok {
		if err := marshaler.UnmarshalMap(value, &m); err != nil {
			return false
		}
	} else {
		if err := json.Unmarshal(value, &m); err != nil {
			return false
		}
	}

	for _, condition := range conditions {
		switch condition.Type {
		case QueryConditionTypeEq:
			if !compareValues(m[condition.Field], condition.Value) {
				return false
			}
		case QueryConditionTypeNe:
			if compareValues(m[condition.Field], condition.Value) {
				return false
			}
		case QueryConditionTypeGt:
			if recordVal, ok := getFloat(m[condition.Field]); ok {
				if val, ok := getFloat(condition.Value); ok {
					if recordVal <= val {
						return false
					}
				}
			} else {
				fieldStr := fmt.Sprintf("%v", m[condition.Field])
				condStr := fmt.Sprintf("%v", condition.Value)
				if fieldStr <= condStr {
					return false
				}
			}
		case QueryConditionTypeGte:
			if recordVal, ok := getFloat(m[condition.Field]); ok {
				if val, ok := getFloat(condition.Value); ok {
					if recordVal < val {
						return false
					}
				}
			} else {
				fieldStr := fmt.Sprintf("%v", m[condition.Field])
				condStr := fmt.Sprintf("%v", condition.Value)
				if fieldStr < condStr {
					return false
				}
			}
		case QueryConditionTypeLt:
			if recordVal, ok := getFloat(m[condition.Field]); ok {
				if val, ok := getFloat(condition.Value); ok {
					if recordVal >= val {
						return false
					}
				}
			} else {
				fieldStr := fmt.Sprintf("%v", m[condition.Field])
				condStr := fmt.Sprintf("%v", condition.Value)
				if fieldStr >= condStr {
					return false
				}
			}
		case QueryConditionTypeLte:
			if recordVal, ok := getFloat(m[condition.Field]); ok {
				if val, ok := getFloat(condition.Value); ok {
					if recordVal > val {
						return false
					}
				}
			} else {
				fieldStr := fmt.Sprintf("%v", m[condition.Field])
				condStr := fmt.Sprintf("%v", condition.Value)
				if fieldStr > condStr {
					return false
				}
			}
		case QueryConditionTypeIn:
			s, ok := getSliceString(condition.Value)
			if !ok {
				return false
			}
			fieldStr := fmt.Sprintf("%v", m[condition.Field])
			if !slices.Contains(s, fieldStr) {
				return false
			}
		case QueryConditionTypeNotIn:
			s, ok := getSliceString(condition.Value)
			if !ok {
				return false
			}
			fieldStr := fmt.Sprintf("%v", m[condition.Field])
			if slices.Contains(s, fieldStr) {
				return false
			}
		case QueryConditionTypeLike:
			fieldStr := fmt.Sprintf("%v", m[condition.Field])
			if !strings.Contains(fieldStr, fmt.Sprintf("%v", condition.Value)) {
				return false
			}
		case QueryConditionTypeNotLike:
			fieldStr := fmt.Sprintf("%v", m[condition.Field])
			if strings.Contains(fieldStr, fmt.Sprintf("%v", condition.Value)) {
				return false
			}
		case QueryConditionTypeIsNull:
			if m[condition.Field] != nil {
				return false
			}
		case QueryConditionTypeIsNotNull:
			if m[condition.Field] == nil {
				return false
			}
		case QueryConditionTypeBetween:
			if recordVal, ok := getInt64(m[condition.Field]); ok {
				s, ok := getSlice(condition.Value)
				if !ok {
					return false
				}
				if start, ok := getInt64(s[0]); ok {
					if end, ok := getInt64(s[1]); ok {
						if recordVal < start {
							return false
						}
						if recordVal >= end {
							return false
						}
					}
				}
			}
		case QueryConditionTypeNotBetween:
			if recordVal, ok := getInt64(m[condition.Field]); ok {
				s, ok := getSlice(condition.Value)
				if !ok {
					return false
				}
				if start, ok := getInt64(s[0]); ok {
					if end, ok := getInt64(s[1]); ok {
						if recordVal >= start && recordVal < end {
							return false
						}
					}
				}
			}
		case QueryConditionTypeAnd:
			if sub, ok := condition.Value.([]QueryCondition); ok {
				if !filterConditionsWithMap(ctx, marshaler, value, sub...) {
					return false
				}
			}
		case QueryConditionTypeOr:
			if sub, ok := condition.Value.([]QueryCondition); ok {
				matched := false
				for _, c := range sub {
					if filterConditionsWithMap(ctx, marshaler, value, c) {
						matched = true
						break
					}
				}
				if !matched {
					return false
				}
			}
		case QueryConditionTypeDistinct:
			// distinct 在 List 层面处理，filter 阶段跳过
			continue
		case QueryConditionTypeSkip:
			continue
		default:
			logger.Warn(ctx, "unknown query condition type", zap.String("type", string(condition.Type)))
		}
	}

	return true // 所有条件都匹配，不过滤
}
func filterConditionsWithGjson(ctx context.Context, value []byte, conditions ...QueryCondition) bool {
	json := string(value)
	for _, condition := range conditions {
		switch condition.Type {
		case QueryConditionTypeEq:
			if gjson.Get(json, condition.Field).String() != fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeNe:
			if gjson.Get(json, condition.Field).String() == fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeGt:
			recordVal := gjson.Get(json, condition.Field).Float()
			if condVal, ok := getFloat(condition.Value); ok {
				if recordVal <= condVal {
					return false
				}
			} else if gjson.Get(json, condition.Field).String() <= fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeGte:
			recordVal := gjson.Get(json, condition.Field).Float()
			if condVal, ok := getFloat(condition.Value); ok {
				if recordVal < condVal {
					return false
				}
			} else if gjson.Get(json, condition.Field).String() < fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeLt:
			recordVal := gjson.Get(json, condition.Field).Float()
			if condVal, ok := getFloat(condition.Value); ok {
				if recordVal >= condVal {
					return false
				}
			} else if gjson.Get(json, condition.Field).String() >= fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeLte:
			recordVal := gjson.Get(json, condition.Field).Float()
			if condVal, ok := getFloat(condition.Value); ok {
				if recordVal > condVal {
					return false
				}
			} else if gjson.Get(json, condition.Field).String() > fmt.Sprintf("%v", condition.Value) {
				return false
			}
		case QueryConditionTypeIn:
			s, ok := getSliceString(condition.Value)
			if !ok {
				return false
			}
			if !slices.Contains(s, gjson.Get(json, condition.Field).String()) {
				return false
			}
		case QueryConditionTypeNotIn:
			s, ok := getSliceString(condition.Value)
			if !ok {
				return false
			}
			if slices.Contains(s, gjson.Get(json, condition.Field).String()) {
				return false
			}
		case QueryConditionTypeLike:
			if !strings.Contains(gjson.Get(json, condition.Field).String(), fmt.Sprintf("%v", condition.Value)) {
				return false
			}
		case QueryConditionTypeNotLike:
			if strings.Contains(gjson.Get(json, condition.Field).String(), fmt.Sprintf("%v", condition.Value)) {
				return false
			}
		case QueryConditionTypeIsNull:
			if gjson.Get(json, condition.Field).Exists() {
				return false
			}
		case QueryConditionTypeIsNotNull:
			if !gjson.Get(json, condition.Field).Exists() {
				return false
			}
		case QueryConditionTypeBetween:
			recordVal := gjson.Get(json, condition.Field).Int()
			s, ok := getSlice(condition.Value)
			if !ok {
				return false
			}
			if start, ok := getInt64(s[0]); ok {
				if end, ok := getInt64(s[1]); ok {
					if recordVal < start {
						return false
					}
					if recordVal >= end {
						return false
					}
				}
			}
		case QueryConditionTypeNotBetween:
			recordVal := gjson.Get(json, condition.Field).Int()
			s, ok := getSlice(condition.Value)
			if !ok {
				return false
			}
			if start, ok := getInt64(s[0]); ok {
				if end, ok := getInt64(s[1]); ok {
					if recordVal >= start && recordVal < end {
						return false
					}
				}
			}
		case QueryConditionTypeAnd:
			if sub, ok := condition.Value.([]QueryCondition); ok {
				if !filterConditionsWithGjson(ctx, value, sub...) {
					return false
				}
			}
		case QueryConditionTypeOr:
			if sub, ok := condition.Value.([]QueryCondition); ok {
				matched := false
				for _, c := range sub {
					if filterConditionsWithGjson(ctx, value, c) {
						matched = true
						break
					}
				}
				if !matched {
					return false
				}
			}
		case QueryConditionTypeDistinct:
			// distinct 在 List 层面处理，filter 阶段跳过
			continue
		case QueryConditionTypeSkip:
			continue
		default:
			logger.Warn(ctx, "unknown query condition type", zap.String("type", string(condition.Type)))
		}
	}
	return true
}

// getDistinctFields 从条件中提取 distinct 指定的字段列表
func getDistinctFields(conditions []QueryCondition) []string {
	for _, c := range conditions {
		if c.Type == QueryConditionTypeDistinct {
			if fields, ok := getSliceString(c.Value); ok {
				return fields
			}
		}
	}
	return nil
}

// buildDistinctKey 按指定字段值生成去重 key
func buildDistinctKey(data any, fields []string) string {
	rv := reflect.ValueOf(data)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		fv := rv.FieldByName(f)
		if fv.IsValid() {
			parts = append(parts, fmt.Sprintf("%v", fv.Interface()))
		}
	}
	return strings.Join(parts, "|")
}

func compareValues(a, b any) bool {
	if a == b {
		return true
	}

	// 尝试数值比较
	if aFloat, aOk := getFloat(a); aOk {
		if bFloat, bOk := getFloat(b); bOk {
			return aFloat == bFloat
		}
	}

	// 字符串比较
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func getInt64(value any) (int64, bool) {
	switch t := value.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int16:
		return int64(t), true
	case int8:
		return int64(t), true
	case uint64:
		return int64(t), true
	case uint:
		return int64(t), true
	case uint32:
		return int64(t), true
	case uint16:
		return int64(t), true
	case uint8:
		return int64(t), true
	}
	return 0, false
}

func getSliceString(value any) ([]string, bool) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	res := make([]string, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		res[i] = rv.Index(i).String()
	}
	return res, true
}
