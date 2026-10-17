package taskmgr

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextGet_String_FromBytes 测试从[]byte获取string
// 这个测试验证可以通过JSON反序列化将[]byte转换为string
func TestContextGet_String_FromBytes(t *testing.T) {
	type Nested struct {
		Value string `json:"value"`
	}
	nested := &Nested{Value: "test"}
	nestedBytes, err := json.Marshal(nested)
	require.NoError(t, err)

	context := &ContextMap{}
	context.Set("test", nestedBytes)
	value, ok := ContextGet[Nested](context, "test")
	assert.True(t, ok)
	assert.Equal(t, nested, &value)
}

// TestContextGet_String_Direct 测试直接获取string
func TestContextGet_String_Direct(t *testing.T) {
	context := &ContextMap{}
	context.Set("test", "test")
	value, ok := ContextGet[string](context, "test")
	assert.True(t, ok)
	assert.Equal(t, "test", value)
}

// TestContextGet_Bytes_Direct 测试直接获取[]byte
func TestContextGet_Bytes_Direct(t *testing.T) {
	context := &ContextMap{}
	expected := []byte("test")
	context.Set("test", expected)
	value, ok := ContextGet[[]byte](context, "test")
	assert.True(t, ok)
	assert.Equal(t, expected, value)
}

// TestContextGet_Bytes_FromString 测试从string获取[]byte
func TestContextGet_Bytes_FromString(t *testing.T) {
	context := &ContextMap{}
	context.Set("test", "test")
	value, ok := ContextGet[[]byte](context, "test")
	assert.True(t, ok)
	assert.Equal(t, []byte("test"), value)
}

// TestContextGet_Bool 测试获取bool类型
func TestContextGet_Bool(t *testing.T) {
	context := &ContextMap{}
	context.Set("true_key", true)
	context.Set("false_key", false)

	trueValue, ok := ContextGet[bool](context, "true_key")
	assert.True(t, ok)
	assert.True(t, trueValue)

	falseValue, ok := ContextGet[bool](context, "false_key")
	assert.True(t, ok)
	assert.False(t, falseValue)
}

// TestContextGet_Int64 测试获取int64类型
func TestContextGet_Int64(t *testing.T) {
	context := &ContextMap{}
	context.Set("int64_key", int64(9223372036854775807))
	context.Set("int64_small", int64(42))

	int64Value, ok := ContextGet[int64](context, "int64_key")
	assert.True(t, ok)
	assert.Equal(t, int64(9223372036854775807), int64Value)

	smallValue, ok := ContextGet[int64](context, "int64_small")
	assert.True(t, ok)
	assert.Equal(t, int64(42), smallValue)
}

// TestContextGet_Uint64 测试获取uint64类型
func TestContextGet_Uint64(t *testing.T) {
	context := &ContextMap{}
	context.Set("uint64_key", uint64(18446744073709551615))
	context.Set("uint64_small", uint64(42))

	uint64Value, ok := ContextGet[uint64](context, "uint64_key")
	assert.True(t, ok)
	assert.Equal(t, uint64(18446744073709551615), uint64Value)

	smallValue, ok := ContextGet[uint64](context, "uint64_small")
	assert.True(t, ok)
	assert.Equal(t, uint64(42), smallValue)
}

// TestContextGet_Float64 测试获取float64类型
func TestContextGet_Float64(t *testing.T) {
	context := &ContextMap{}
	context.Set("float64_key", float64(2.71828))
	context.Set("float64_zero", float64(0.0))

	float64Value, ok := ContextGet[float64](context, "float64_key")
	assert.True(t, ok)
	assert.Equal(t, float64(2.71828), float64Value)

	zeroValue, ok := ContextGet[float64](context, "float64_zero")
	assert.True(t, ok)
	assert.Equal(t, float64(0.0), zeroValue)
}

// TestContextGet_Struct_FromBytes 测试从[]byte反序列化struct
func TestContextGet_Struct_FromBytes(t *testing.T) {
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	user := &User{
		Name: "test",
		Age:  18,
	}
	userBytes, err := json.Marshal(user)
	require.NoError(t, err)

	context := &ContextMap{}
	context.Set("test", userBytes)
	value, ok := ContextGet[User](context, "test")
	assert.True(t, ok)
	assert.Equal(t, user, &value)
}

// TestContextGet_Struct_FromString 测试从string反序列化struct
func TestContextGet_Struct_FromString(t *testing.T) {
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	user := &User{
		Name: "test",
		Age:  18,
	}
	userStr, err := json.Marshal(user)
	require.NoError(t, err)

	context := &ContextMap{}
	context.Set("test", string(userStr))
	value, ok := ContextGet[User](context, "test")
	assert.True(t, ok)
	assert.Equal(t, user, &value)
}

// TestContextGet_Struct_Direct 测试直接获取struct
func TestContextGet_Struct_Direct(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}
	user := User{
		Name: "test",
		Age:  18,
	}
	context := &ContextMap{}
	context.Set("test", user)
	value, ok := ContextGet[User](context, "test")
	assert.True(t, ok)
	assert.Equal(t, user, value)
}

// TestContextGet_Pointer 测试获取指针类型
func TestContextGet_Pointer(t *testing.T) {
	type User struct {
		Name string
	}
	user := &User{Name: "test"}
	context := &ContextMap{}
	context.Set("test", user)
	value, ok := ContextGet[*User](context, "test")
	assert.True(t, ok)
	assert.Equal(t, user, value)
}

// TestContextGet_Nil 测试nil上下文
func TestContextGet_Nil(t *testing.T) {
	value, ok := ContextGet[string](nil, "test")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

// TestContextGet_NotFound 测试不存在的键
func TestContextGet_NotFound(t *testing.T) {
	context := &ContextMap{}
	value, ok := ContextGet[string](context, "test")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

// TestContextGet_InvalidJSON 测试无效的JSON
func TestContextGet_InvalidJSON(t *testing.T) {
	context := &ContextMap{}
	context.Set("test", "invalid json")
	value, ok := ContextGet[map[string]any](context, "test")
	assert.False(t, ok)
	assert.Equal(t, map[string]any(nil), value)
}

// TestContextSet 测试设置值
func TestContextSet(t *testing.T) {
	context := &ContextMap{}
	ContextSet(context, "string_key", "test_value")
	ContextSet(context, "int_key", int64(42))
	ContextSet(context, "uint_key", uint64(100))
	ContextSet(context, "bool_key", true)
	ContextSet(context, "float_key", float64(3.14))
	ContextSet(context, "struct_key", struct{ Name string }{Name: "test"})

	value, ok := ContextGet[string](context, "string_key")
	assert.True(t, ok)
	assert.Equal(t, "test_value", value)

	intValue, ok := ContextGet[int64](context, "int_key")
	assert.True(t, ok)
	assert.Equal(t, int64(42), intValue)

	uintValue, ok := ContextGet[uint64](context, "uint_key")
	assert.True(t, ok)
	assert.Equal(t, uint64(100), uintValue)

	boolValue, ok := ContextGet[bool](context, "bool_key")
	assert.True(t, ok)
	assert.True(t, boolValue)

	floatValue, ok := ContextGet[float64](context, "float_key")
	assert.True(t, ok)
	assert.Equal(t, float64(3.14), floatValue)
}

// TestContextDelete 测试删除键
func TestContextDelete(t *testing.T) {
	context := &ContextMap{}
	context.Set("test", "value")

	// 确认值存在
	value, ok := ContextGet[string](context, "test")
	assert.True(t, ok)
	assert.Equal(t, "value", value)

	// 删除键
	ContextDelete(context, "test")

	// 确认值不存在
	value, ok = ContextGet[string](context, "test")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

// TestContextDelete_Nil 测试删除nil上下文的键
func TestContextDelete_Nil(t *testing.T) {
	ContextDelete(nil, "test")
	assert.NotPanics(t, func() {
		ContextDelete(nil, "test")
	})
}

// TestContextToMap 测试转换为map
func TestContextToMap(t *testing.T) {
	context := &ContextMap{}
	context.Set("string_key", "value1")
	context.Set("int_key", 42)
	context.Set("bool_key", true)

	result := ContextToMap(context)
	assert.Len(t, result, 3)
	assert.Equal(t, "value1", result["string_key"])
	assert.Equal(t, 42, result["int_key"])
	assert.Equal(t, true, result["bool_key"])
}

// TestContextToMap_Nil 测试nil上下文转换为map
func TestContextToMap_Nil(t *testing.T) {
	result := ContextToMap(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// TestContextToMap_Empty 测试空上下文转换为map
func TestContextToMap_Empty(t *testing.T) {
	context := &ContextMap{}
	result := ContextToMap(context)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// TestUpdatePosition 测试更新位置
func TestUpdatePosition(t *testing.T) {
	var calledPosition int
	var called bool

	ctx := genUpdatePositionCtx(context.Background(), func(position int) {
		called = true
		calledPosition = position
	})

	UpdatePosition(ctx, 123)
	assert.True(t, called)
	assert.Equal(t, 123, calledPosition)
}

// TestUpdatePosition_NoFunc 测试没有更新函数的上下文
func TestUpdatePosition_NoFunc(t *testing.T) {
	ctx := context.Background()
	// 不应该panic
	UpdatePosition(ctx, 123)
}

// TestGenUpdatePositionCtx 测试生成更新位置上下文
func TestGenUpdatePositionCtx(t *testing.T) {
	var called bool
	ctx := genUpdatePositionCtx(context.Background(), func(position int) {
		called = true
	})
	assert.NotNil(t, ctx)

	UpdatePosition(ctx, 1)
	assert.True(t, called)
}
