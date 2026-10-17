package dataaccess

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type BaseType interface {
	~string | ~int | ~int64 | ~float64 | ~bool | time.Time
}
type BaseTypePtr interface {
	~*string | ~*int | ~*int64 | ~*float64 | ~*bool | ~*time.Time
}

type BaseTypeAndPtr interface {
	BaseType | BaseTypePtr
}

func Eq[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeEq,
		Field: field,
		Value: value,
	}
}
func IfEq[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeEq,
		Field: field,
		Value: value,
	}
}

func Ne[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeNe,
		Field: field,
		Value: value,
	}
}

// EqF - 使用字段函数的Eq
func EqF[T BaseType](fieldFunc func(table string, joinTables ...string) string, value T) QueryCondition {
	return QueryCondition{
		Type:      QueryConditionTypeEq,
		FieldFunc: fieldFunc,
		Value:     value,
	}
}

// GtF - 使用字段函数的Gt
func GtF[T BaseType](fieldFunc func(table string, joinTables ...string) string, value T) QueryCondition {
	return QueryCondition{
		Type:      QueryConditionTypeGt,
		FieldFunc: fieldFunc,
		Value:     value,
	}
}

func IfNe[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeNe,
		Field: field,
		Value: value,
	}
}
func Gt[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeGt,
		Field: field,
		Value: value,
	}
}
func IfGt[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeGt,
		Field: field,
		Value: value,
	}
}

func Gte[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeGte,
		Field: field,
		Value: value,
	}
}
func IfGte[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeGte,
		Field: field,
		Value: value,
	}
}

func Lt[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeLt,
		Field: field,
		Value: value,
	}
}

func IfLt[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeLt,
		Field: field,
		Value: value,
	}
}

func Lte[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeLte,
		Field: field,
		Value: value,
	}
}

func IfLte[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeLte,
		Field: field,
		Value: value,
	}
}

func In[T BaseType](field string, values []T) QueryCondition {
	set := map[T]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	newValues := make([]T, 0, len(set))
	for value := range set {
		newValues = append(newValues, value)
	}
	return QueryCondition{
		Type:  QueryConditionTypeIn,
		Field: field,
		Value: newValues,
	}
}

func IfIn[T BaseTypeAndPtr](field string, values []T) QueryCondition {
	if len(values) == 0 {
		return Skip()
	}
	set := map[T]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	newValues := make([]T, 0, len(set))
	for value := range set {
		newValues = append(newValues, value)
	}
	return QueryCondition{
		Type:  QueryConditionTypeIn,
		Field: field,
		Value: newValues,
	}
}

func NotIn[T BaseType](field string, values []T) QueryCondition {
	set := map[T]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	newValues := make([]T, 0, len(set))
	for value := range set {
		newValues = append(newValues, value)
	}
	return QueryCondition{
		Type:  QueryConditionTypeNotIn,
		Field: field,
		Value: newValues,
	}
}

func IfNotIn[T BaseTypeAndPtr](field string, values []T) QueryCondition {
	if len(values) == 0 {
		return Skip()
	}
	set := map[T]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	newValues := make([]T, 0, len(set))
	for value := range set {
		newValues = append(newValues, value)
	}
	return QueryCondition{
		Type:  QueryConditionTypeNotIn,
		Field: field,
		Value: newValues,
	}
}

func Like[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeLike,
		Field: field,
		Value: value,
	}
}

func IfLike[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeLike,
		Field: field,
		Value: value,
	}
}

func NotLike[T BaseType](field string, value T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeNotLike,
		Field: field,
		Value: value,
	}
}

func IfNotLike[T BaseTypeAndPtr](field string, value T) QueryCondition {
	if reflect.ValueOf(value).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeNotLike,
		Field: field,
		Value: value,
	}
}
func IsNull(field string) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeIsNull,
		Field: field,
	}
}

func IsNotNull(field string) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeIsNotNull,
		Field: field,
	}
}

func Between[T BaseType](field string, start, end T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeBetween,
		Field: field,
		Value: []any{start, end},
	}
}
func IfBetween[T BaseTypeAndPtr](field string, start, end T) QueryCondition {
	if reflect.ValueOf(start).IsZero() || reflect.ValueOf(end).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeBetween,
		Field: field,
		Value: []any{start, end},
	}
}

func NotBetween[T BaseType](field string, start, end T) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeNotBetween,
		Field: field,
		Value: []any{start, end},
	}
}

func IfNotBetween[T BaseTypeAndPtr](field string, start, end T) QueryCondition {
	if reflect.ValueOf(start).IsZero() || reflect.ValueOf(end).IsZero() {
		return Skip()
	}
	return QueryCondition{
		Type:  QueryConditionTypeNotBetween,
		Field: field,
		Value: []any{start, end},
	}
}

type OrderType string

const (
	OrderTypeAsc  OrderType = "ASC"
	OrderTypeDesc OrderType = "DESC"
)

type JoinType string

const (
	JoinTypeLeft  JoinType = "LEFT"
	JoinTypeRight JoinType = "RIGHT"
	JoinTypeInner JoinType = "INNER"
)

// joinOn - JOIN ON 条件，私有类型
type joinOn struct {
	Table1Field string
	Table2Field string
}

// JoinOn - 构造函数
func JoinOn(table1Field, table2Field string) *joinOn {
	return &joinOn{Table1Field: table1Field, Table2Field: table2Field}
}

// SF - 生成主表字段的函数，默认使用第一个表(主表)
func SF(field string) SelectFunc {
	return func(table string, joinTables ...string) string {
		return table + "." + field
	}
}

// SFN - 生成指定表字段的函数，第一个参数为表索引(0=主表, 1+=JOIN表)
func SFN(n int, field string) SelectFunc {
	if n <= 0 {
		return SF(field)
	}
	return func(table string, joinTables ...string) string {
		if len(joinTables) >= n {
			return joinTables[n-1] + "." + field
		}
		return field
	}
}

// OrderCase - 按指定字段的枚举值顺序排序，未命中的值排最后
func OrderCase(field string, values ...string) SelectFunc {
	return OrderCaseF(SF(field), OrderTypeAsc, values...)
}

// OrderCaseDesc - 按指定字段的枚举值顺序倒序排序，未命中的值排最后
func OrderCaseDesc(field string, values ...string) SelectFunc {
	return OrderCaseF(SF(field), OrderTypeDesc, values...)
}

// OrderCaseF - 按字段函数的枚举值顺序排序，支持 JOIN 表字段
func OrderCaseF(fieldFunc SelectFunc, direction OrderType, values ...string) SelectFunc {
	return func(table string, joinTables ...string) string {
		fieldName := fieldFunc(table, joinTables...)
		if len(values) == 0 {
			return fmt.Sprintf("%s %s", fieldName, direction)
		}

		var b strings.Builder
		b.WriteString("CASE ")
		b.WriteString(fieldName)
		for i, value := range values {
			b.WriteString(" WHEN ")
			b.WriteString(sqlString(value))
			b.WriteString(" THEN ")
			b.WriteString(fmt.Sprintf("%d", i+1))
		}
		elseRank := len(values) + 1
		if direction == OrderTypeDesc {
			elseRank = 0
		}
		b.WriteString(" ELSE ")
		b.WriteString(fmt.Sprintf("%d", elseRank))
		b.WriteString(" END ")
		b.WriteString(string(direction))
		return b.String()
	}
}

func sqlString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func Order(field string, direction OrderType) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeOrder,
		Field: field,
		Value: direction,
	}
}

func Limit(limit int) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeLimit,
		Field: "limit",
		Value: limit,
	}
}

func Offset(offset int) QueryCondition {
	return QueryCondition{
		Type:  QueryConditionTypeOffset,
		Field: "offset",
		Value: offset,
	}
}

type QueryConditionType string

const (
	QueryConditionTypeSkip       QueryConditionType = "skip"
	QueryConditionTypeOrder      QueryConditionType = "order"
	QueryConditionTypeLimit      QueryConditionType = "limit"
	QueryConditionTypeOffset     QueryConditionType = "offset"
	QueryConditionTypeEq         QueryConditionType = "eq"
	QueryConditionTypeNe         QueryConditionType = "ne"
	QueryConditionTypeGt         QueryConditionType = "gt"
	QueryConditionTypeGte        QueryConditionType = "gte"
	QueryConditionTypeLt         QueryConditionType = "lt"
	QueryConditionTypeLte        QueryConditionType = "lte"
	QueryConditionTypeIn         QueryConditionType = "in"
	QueryConditionTypeNotIn      QueryConditionType = "notIn"
	QueryConditionTypeLike       QueryConditionType = "like"
	QueryConditionTypeNotLike    QueryConditionType = "notLike"
	QueryConditionTypeIsNull     QueryConditionType = "isNull"
	QueryConditionTypeIsNotNull  QueryConditionType = "isNotNull"
	QueryConditionTypeBetween    QueryConditionType = "between"
	QueryConditionTypeNotBetween QueryConditionType = "notBetween"
	QueryConditionTypeAnd        QueryConditionType = "and"
	QueryConditionTypeOr         QueryConditionType = "or"
	QueryConditionTypeDistinct   QueryConditionType = "distinct"
)

type QueryCondition struct {
	Type      QueryConditionType                              // 查询类型：eq, ne, gt, gte, lt, lte, in, notIn, like, notLike, isNull, isNotNull, between, notBetween
	Field     string                                          // 字段名
	FieldFunc func(table string, joinTables ...string) string // 字段函数(可选，支持指定表)
	Value     any                                             // 查询值
}

func (q QueryCondition) IsSkip() bool {
	return q.Type == QueryConditionTypeSkip
}
func Skip() QueryCondition {
	return QueryCondition{Type: QueryConditionTypeSkip}
}

// And 将多个条件组合为 AND 关系，可嵌套在 Or 中使用
func And(conditions ...QueryCondition) QueryCondition {
	return QueryCondition{Type: QueryConditionTypeAnd, Value: conditions}
}

// Or 将多个条件组合为 OR 关系
func Or(conditions ...QueryCondition) QueryCondition {
	return QueryCondition{Type: QueryConditionTypeOr, Value: conditions}
}

// Distinct 去重查询
func Distinct(fields ...string) QueryCondition {
	return QueryCondition{Type: QueryConditionTypeDistinct, Value: fields}
}
func Page(page, size int) []QueryCondition {
	if page <= 0 || size <= 0 {
		return []QueryCondition{}
	}
	return []QueryCondition{
		Limit(size),
		Offset((page - 1) * size),
	}
}
func (q QueryCondition) String() string {
	return fmt.Sprintf("field:%s type:%s value:%v", q.Field, q.Type, q.Value)
}

type QueryConditions []QueryCondition

func (q QueryConditions) String() string {
	if len(q) == 0 {
		return ""
	}
	if len(q) == 1 {
		return q[0].String()
	}
	newQ := make([]string, 0, len(q))
	for _, condition := range q {
		if condition.IsSkip() {
			continue
		}
		newQ = append(newQ, condition.String())
	}
	return strings.Join(newQ, " && ")
}
