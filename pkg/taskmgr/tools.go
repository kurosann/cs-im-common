package taskmgr

import (
	"context"
	"encoding/json"
	"reflect"
)

type taskContextKey string

const (
	updatePositionFuncKey taskContextKey = "updatePositionFunc"
	updateResultFuncKey   taskContextKey = "updateResultFunc"
)

func UpdatePosition(ctx context.Context, position int) {
	updatePositionFunc, ok := ctx.Value(updatePositionFuncKey).(func(position int))
	if !ok {
		return
	}
	updatePositionFunc(position)
}
func genUpdatePositionCtx(ctx context.Context, fn func(position int)) context.Context {
	return context.WithValue(ctx, updatePositionFuncKey, fn)
}
func UpdateResult(ctx context.Context, result []byte) {
	updateResultFunc, ok := ctx.Value(updateResultFuncKey).(func(result []byte))
	if !ok {
		return
	}
	updateResultFunc(result)
}
func genUpdateResultCtx(ctx context.Context, fn func(result []byte)) context.Context {
	return context.WithValue(ctx, updateResultFuncKey, fn)
}

// T的int不能是 int8, int16, int32, uint8, uint16, uint32, float32
func ContextGet[T any](context *ContextMap, key string) (T, bool) {
	var zeroValue T
	if context == nil {
		return zeroValue, false
	}
	v, ok := context.Get(key)
	if !ok {
		return zeroValue, false
	}
	switch any(zeroValue).(type) {
	case string:
		switch data := v.(type) {
		case string:
			return v.(T), true
		case []byte:
			return any(string(data)).(T), true
		default:
			return zeroValue, false
		}
	case []byte:
		switch data := v.(type) {
		case string:
			return any([]byte(data)).(T), true
		case []byte:
			return v.(T), true
		default:
			return zeroValue, false
		}
	case bool:
		if rv := reflect.ValueOf(v); rv.Kind() == reflect.Bool {
			return any(rv.Bool()).(T), true
		}
	case int, int64:
		if rv := reflect.ValueOf(v); rv.CanInt() {
			return any(rv.Int()).(T), true
		}
	case uint, uint64:
		if rv := reflect.ValueOf(v); rv.CanUint() {
			return any(rv.Uint()).(T), true
		}
	case float64:
		if rv := reflect.ValueOf(v); rv.CanFloat() {
			return any(rv.Float()).(T), true
		}
	}
	switch data := v.(type) {
	case T:
		return data, true
	case []byte:
		err := json.Unmarshal(data, &zeroValue)
		if err != nil {
			return zeroValue, false
		}
		return zeroValue, true
	case string:
		err := json.Unmarshal([]byte(data), &zeroValue)
		if err != nil {
			return zeroValue, false
		}
		return zeroValue, true
	case map[string]any:
		// 如果存储的是 map[string]any，先转换为 JSON 再反序列化
		jsonData, err := json.Marshal(data)
		if err != nil {
			return zeroValue, false
		}
		err = json.Unmarshal(jsonData, &zeroValue)
		if err != nil {
			return zeroValue, false
		}
		return zeroValue, true
	}
	return zeroValue, false
}

func ContextSet[T any](context *ContextMap, key string, value T) {
	if context == nil {
		return
	}
	context.Set(key, value)
}

func ContextDelete(context *ContextMap, key string) {
	if context == nil {
		return
	}
	context.Delete(key)
}

func ContextToMap(context *ContextMap) map[string]any {
	if context == nil {
		return make(map[string]any)
	}
	result := make(map[string]any)
	context.Range(func(key string, value any) bool {
		result[key] = value
		return true
	})
	return result
}
