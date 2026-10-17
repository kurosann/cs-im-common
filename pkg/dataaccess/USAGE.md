# dataaccess 使用文档

dataaccess 是一个统一的数据访问层组件，通过泛型接口屏蔽了底层存储引擎（关系型数据库 / KV 数据库）的差异，提供一致的 CRUD、查询、缓存、事件监听和事务能力。

---

## 目录

- [架构概览](#架构概览)
- [快速开始](#快速开始)
- [定义模型](#定义模型)
- [创建访问器](#创建访问器)
- [CRUD 操作](#crud-操作)
- [查询条件](#查询条件)
- [更新选项](#更新选项)
- [TTL 过期机制](#ttl-过期机制)
- [乐观锁 (Revision)](#乐观锁-revision)
- [缓存](#缓存)
- [事件监听](#事件监听)
- [事务](#事务)
- [存储注册](#存储注册)
- [错误处理](#错误处理)
- [扩展自定义存储引擎](#扩展自定义存储引擎)

---

## 架构概览

```
┌─────────────────────────────────────┐
│         DataAccessor[T] (门面层)      │  ← 用户直接使用，内置缓存 + 事件广播
├──────────────┬──────────────────────┤
│ GormDataAccessor  │  KvDBDataAccessor │  ← 引擎实现层
│   (关系型 DB)      │   (KV DB)        │
├──────────────┴──────────────────────┤
│           store (存储注册层)           │  ← 注册 RelationDB / KVDB 实例
└─────────────────────────────────────┘
```

**核心思路**：模型通过 `DbType()` 声明自己使用哪种存储引擎，`DataAccessor` 根据该类型自动路由到对应实现。

---

## 快速开始

### 1. 定义模型

嵌入 `base.BaseModel` 即可获得 ID、时间戳等基础字段，并自动满足 `model.Table` 接口。

```go
type User struct {
    base.BaseModel                        // 提供 ID / CreateAt / UpdateAt 等
    Username string `json:"username" gorm:"column:username"`
    Email    string `json:"email" gorm:"column:email"`
    Status   int    `json:"status" gorm:"column:status"`
}

func (u *User) TableName() string { return "users" }

// 可选：覆盖默认数据源（默认 "main"）
func (u *User) DbSource() model.DbSource { return "main" }

// 可选：覆盖存储类型（默认 DBTypeRelation）
func (u *User) DbType() model.DBType { return model.DBTypeRelation }
```

### 2. 注册数据库

```go
import "vane-common/pkg/dataaccess/store"

// 关系型数据库
store.RegisterRelationDB("main", store.RelationStore{DB: gormDB})

// KV 数据库
store.RegisterKVDB("cache_db", myKVStore)  // myKVStore 需实现 store.IKVStore
```

### 3. 使用

```go
accessor := dataaccess.Accessor[*User]("")

// 创建
user := &User{Username: "alice", Email: "alice@example.com"}
err := accessor.Create(ctx, user)

// 查询
result, err := accessor.GetByID(ctx, field.ID(user.ID))

// 删除
err = accessor.DeleteById(ctx, field.ID(user.ID))
```

---

## 定义模型

### model.Table 接口

所有模型必须实现以下接口：

```go
type Table interface {
    TableName() string          // 表名 / KV 前缀
    GetID() field.IID           // 获取主键
    SetID(field.IID)            // 设置主键
    IDColumn() string           // 主键列名（如 "id"）
    Create()                    // 创建时回调（通常设置时间戳）
    Update()                    // 更新时回调（通常设置时间戳）
    DbSource() DbSource         // 数据源名称
    DbType() DBType             // 存储类型：DBTypeRelation 或 DBTypeKv
}
```

### 可选接口（通过接口断言自动生效）

| 接口 | 作用 | 嵌入结构体 |
|------|------|-----------|
| `model.ExpireAtTable` | 支持 TTL 过期 | `base.ExpireAtBaseModel` |
| `model.RevisionTable` | 支持乐观锁 | `base.RevisionBaseModel` |
| `model.OwnerTable` | 数据归属 | `base.OwnerBaseModel` |
| `model.TableMarshaler` | 自定义序列化/反序列化（KV 场景） | 手动实现 |

### 内置 BaseModel

| 结构体 | 说明 |
|--------|------|
| `base.BaseModel` | 包含 `id`（int64 自增主键）、`create_by`、`create_at`、`update_by`、`update_at`，默认 DbSource="main"，DbType=Relation |
| `base.BaseModelOnlyDate` | 同上但不含 ID，适用于自定义主键的场景 |
| `base.ExpireAtBaseModel` | 添加 `expire_at` 字段 |
| `base.RevisionBaseModel` | 添加 `revision` 字段（乐观锁） |
| `base.OwnerBaseModel` | 添加 `owner` 字段 |

### 示例：带过期和乐观锁的模型

```go
type Session struct {
    base.BaseModel
    base.ExpireAtBaseModel
    base.RevisionBaseModel
    Token string `json:"token" gorm:"column:token"`
}

func (s *Session) TableName() string { return "sessions" }
```

### KV 模型示例

```go
type MyKVModel struct {
    base.BaseModelOnlyDate              // 不需要自增 ID
    Key   string `json:"key"`
    Value string `json:"value"`
}

func (m *MyKVModel) TableName() string          { return "my_kv" }
func (m *MyKVModel) GetID() field.IID           { return field.ID(m.Key) }
func (m *MyKVModel) SetID(id field.IID)         { m.Key = id.String() }
func (m *MyKVModel) IDColumn() string           { return "key" }
func (m *MyKVModel) DbSource() model.DbSource   { return "cache_db" }
func (m *MyKVModel) DbType() model.DBType       { return model.DBTypeKv }
```

---

## 创建访问器

### 方式一：门面访问器（推荐）

自带缓存 + 事件广播：

```go
accessor := dataaccess.Accessor[*User]("")
```

### 方式二：直接使用引擎访问器

```go
// 关系型
gormAccessor := &dataaccess.GormDataAccessor[*User]{}

// KV
kvAccessor := &dataaccess.KvDBDataAccessor[*User]{}
```

---

## CRUD 操作

### Create

```go
user := &User{Username: "alice", Email: "alice@example.com"}
err := accessor.Create(ctx, user)
// user.ID 自动填充（关系型 DB 自增）
```

### Get / GetByID

```go
// 通过 ID 查询
result, err := accessor.GetByID(ctx, field.ID(1))

// 通过模型实例查询（使用 data.GetID()）
result, err := accessor.Get(ctx, user)
```

### First

```go
// 返回第一条匹配记录，未找到返回 ErrDataNotFound
user, err := accessor.First(ctx, dataaccess.Eq("username", "alice"))
```

### List

```go
// 无条件查询全部
users, err := accessor.List(ctx)

// 带条件查询
users, err := accessor.List(ctx,
    dataaccess.Eq("status", 1),
    dataaccess.Order("create_at", dataaccess.OrderTypeDesc),
    dataaccess.Limit(10),
)
```

### Count

```go
count, err := accessor.Count(ctx, dataaccess.Eq("status", 1))
```

### Save（创建或更新）

```go
// ID 为空时创建，已有 ID 时更新
err := accessor.Save(ctx, user)
```

### Update

```go
user.Email = "new@example.com"
err := accessor.Update(ctx, user)

// 只更新指定字段
err := accessor.Update(ctx, user, dataaccess.OnlyFields("email", "status"))
```

### Delete / DeleteById

```go
err := accessor.Delete(ctx, user)
err := accessor.DeleteById(ctx, field.ID(1))
```

### Clean（清理过期数据）

仅对实现了 `model.ExpireAtTable` 的模型有效：

```go
err := accessor.Clean(ctx)
```

---

## 查询条件

所有条件函数定义在 `condition.go` 中，返回 `QueryCondition`，用于 `List`、`First`、`Count` 方法。

### 比较条件

| 函数 | SQL 等价 | 说明 |
|------|---------|------|
| `Eq(field, value)` | `field = value` | 等于 |
| `Ne(field, value)` | `field <> value` | 不等于 |
| `Gt(field, value)` | `field > value` | 大于 |
| `Gte(field, value)` | `field >= value` | 大于等于 |
| `Lt(field, value)` | `field < value` | 小于 |
| `Lte(field, value)` | `field <= value` | 小于等于 |

### 集合条件

| 函数 | SQL 等价 | 说明 |
|------|---------|------|
| `In(field, []T)` | `field IN (...)` | 在集合中 |
| `NotIn(field, []T)` | `field NOT IN (...)` | 不在集合中 |

### 模糊匹配

| 函数 | SQL 等价 | 说明 |
|------|---------|------|
| `Like(field, value)` | `field LIKE '%value%'` | 模糊匹配 |
| `NotLike(field, value)` | `field NOT LIKE '%value%'` | 模糊不匹配 |

### 空值判断

| 函数 | SQL 等价 |
|------|---------|
| `IsNull(field)` | `field IS NULL` |
| `IsNotNull(field)` | `field IS NOT NULL` |

### 范围条件

| 函数 | SQL 等价 | 说明 |
|------|---------|------|
| `Between(field, start, end)` | `field BETWEEN start AND end` | 在范围内 |
| `NotBetween(field, start, end)` | `field NOT BETWEEN start AND end` | 不在范围内 |

### 排序与分页

```go
// 排序
dataaccess.Order("create_at", dataaccess.OrderTypeDesc)   // DESC
dataaccess.Order("create_at", dataaccess.OrderTypeAsc)    // ASC

// 分页（Limit + Offset）
dataaccess.Limit(10)
dataaccess.Offset(20)

// 快捷分页（page 从 1 开始）
dataaccess.Page(2, 10)  // 第2页，每页10条 → Limit(10), Offset(10)
```

### 条件前缀 `If` 系列（条件跳过）

当值为零值时自动跳过，用于动态构建查询：

```go
// status 为 0 时自动跳过该条件
users, err := accessor.List(ctx,
    dataaccess.IfEq("status", status),     // status=0 → Skip
    dataaccess.IfLike("name", name),       // name="" → Skip
    dataaccess.IfIn("category_id", ids),   // ids=[] → Skip
)
```

可用的 `If` 系列函数：`IfEq`、`IfNe`、`IfGt`、`IfGte`、`IfLt`、`IfLte`、`IfIn`、`IfNotIn`、`IfLike`、`IfNotLike`、`IfBetween`、`IfNotBetween`。

---

## 更新选项

### OnlyFields — 指定更新字段

```go
err := accessor.Update(ctx, user, dataaccess.OnlyFields("email", "status"))
```

### IfRetry — 乐观锁冲突重试

当模型实现了 `model.RevisionTable` 且更新时 revision 不匹配（`RowsAffected == 0`），自动重试：

```go
err := accessor.Update(ctx, user,
    dataaccess.IfRetry[*User](func(ctx context.Context, latest *User) (*User, error) {
        // latest 是数据库中的最新记录
        latest.Email = "new@example.com"  // 在最新数据上重新应用修改
        return latest, nil
    }),
)
```

### BuildUpdateOpt — 在引擎访问器中使用更新选项

```go
opt := dataaccess.BuildUpdateOpt[*User](dataaccess.OnlyFields("email"))
err := kvAccessor.Update(ctx, user, opt)
```

---

## TTL 过期机制

模型需要嵌入 `base.ExpireAtBaseModel`（或手动实现 `model.ExpireAtTable`）。

```go
// 创建时设置 TTL
err := accessor.CreateWithTtl(ctx, session, 30*time.Minute)

// 更新时刷新 TTL
err := accessor.UpdateWithTtl(ctx, session, 30*time.Minute)
```

- 查询（`Get`、`List`、`First`、`Count`）自动过滤已过期数据
- `Clean()` 方法可手动清理过期记录

---

## 乐观锁 (Revision)

模型需要嵌入 `base.RevisionBaseModel`（或手动实现 `model.RevisionTable`）。

**工作原理**：
1. 更新时自动附加 `WHERE revision = oldRevision` 条件
2. 更新成功后 revision 自动 +1
3. 如果 revision 不匹配（`RowsAffected == 0`），可通过 `IfRetry` 设置重试逻辑
4. revision 达到 `MaxInt64` 时自动归零

```go
type Order struct {
    base.BaseModel
    base.RevisionBaseModel
    Amount float64 `json:"amount" gorm:"column:amount"`
}

// 安全更新（带重试）
err := accessor.Update(ctx, order,
    dataaccess.IfRetry[*Order](func(ctx context.Context, latest *Order) (*Order, error) {
        latest.Amount = newAmount
        return latest, nil
    }),
)
```

---

## 缓存

通过 `EnableCache` 启用，仅对 **门面访问器** (`DataAccessor`) 有效。

```go
// 注册缓存（ttl 为缓存有效期）
dataaccess.EnableCache(&User{}, 5*time.Minute)
```

**行为**：
- `Get`、`GetByID`、`First`、`List`、`Count` 查询时先读缓存，命中则跳过 DB 查询
- `Save`、`Update`、`Delete` 操作后自动清除该表所有查询缓存
- 缓存基于 `github.com/patrickmn/go-cache`（内存缓存）

---

## 事件监听

通过 `RegisterDataListen` 注册，仅对 **门面访问器** 有效。事件通过 goroutine 异步广播。

```go
dataaccess.RegisterDataListen(&User{}, func(item dataaccess.ListenItem[model.Table]) {
    switch item.Action {
    case dataaccess.ListenActionCreate:
        log.Println("用户创建:", item.Model.GetID())
    case dataaccess.ListenActionUpdate:
        log.Println("用户更新:", item.Model.GetID())
    case dataaccess.ListenActionDelete:
        log.Println("用户删除:", item.Model.GetID())
    }
})
```

事件类型：
- `ListenActionCreate` — Create / Save（新建）后触发
- `ListenActionUpdate` — Update / Save（更新）后触发
- `ListenActionDelete` — Delete 后触发

---

## 事务

### 模型级事务（推荐）

通过访问器的 `Transaction` 方法，自动绑定该模型的数据源：

```go
err := accessor.Transaction(ctx, func(txCtx context.Context) error {
    if err := accessor.Create(txCtx, user1); err != nil {
        return err  // 返回 error 自动回滚
    }
    if err := accessor.Create(txCtx, user2); err != nil {
        return err
    }
    return nil  // 返回 nil 自动提交
})
```

### 通用事务

跨存储类型的事务（需注意 KV 和 Relation 不能混合在一个事务中）：

```go
err := dataaccess.Transaction(ctx, model.DBTypeRelation, model.DbSource("main"), func(txCtx context.Context) error {
    // 在同一个关系型 DB 事务中操作
    return accessor.Create(txCtx, user)
})
```

---

## 存储注册

### 关系型数据库

```go
import (
    "vane-common/pkg/dataaccess/store"
    "gorm.io/gorm"
)

var gormDB *gorm.DB = /* 初始化 gorm */

store.RegisterRelationDB("main", store.RelationStore{DB: gormDB})

// 自动迁移
store.RelationDbAutoMigrate(ctx, &User{}, &Product{})

// 清理过期数据
store.RelationDbClean(ctx, &Session{})
```

### KV 数据库

需实现 `store.IKVStore` 接口：

```go
type IKVStore interface {
    Get(ctx context.Context, key []byte) (value []byte, err error)
    Put(ctx context.Context, key []byte, value []byte) error
    PutWithTtl(ctx context.Context, key []byte, value []byte, ttl time.Duration) error
    Scan(ctx context.Context, prefix []byte, filter FilterFunc) (map[string][]byte, error)
    Count(ctx context.Context, prefix []byte, filter FilterFunc) (int64, error)
    Del(ctx context.Context, key []byte) error
    StartTx(ctx context.Context) context.Context
    Transaction(ctx context.Context, fn func(ctx context.Context) error) error
    Clean(ctx context.Context) error
    Close() error
}

store.RegisterKVDB("cache_db", myKVStore)
```

---

## 错误处理

```go
import "vane-common/pkg/dataaccess"

result, err := accessor.GetByID(ctx, id)
if err != nil {
    if errors.Is(err, dataaccess.ErrDataNotFound) {
        // 数据不存在
    }
    if errors.Is(err, dataaccess.ErrIDRequired) {
        // 缺少 ID
    }
    if errors.Is(err, dataaccess.ErrRevisionMismatch) {
        // 乐观锁冲突（事务上下文被取消时也会触发）
    }
}
```

| 错误 | 触发场景 |
|------|---------|
| `ErrDataNotFound` | Get/GetByID/First 未找到记录 |
| `ErrIDRequired` | Update/Delete 时 ID 为空 |
| `ErrRevisionMismatch` | 乐观锁重试时 context 被取消 |

---

## 扩展自定义存储引擎

如果需要支持 Relation 和 KV 之外的存储（如 Redis、MongoDB），可以通过注册工厂函数实现：

```go
// 1. 定义自定义 DBType
const DBTypeRedis model.DBType = "redis"

// 2. 实现 IExtendDataAccessor 接口
type RedisDataAccessor struct {
    constructor func() model.Table
    // ...
}

// 3. 注册工厂函数
dataaccess.RegisterAccessorFactory(DBTypeRedis, func(constructor func() model.Table) dataaccess.IExtendDataAccessor {
    return &RedisDataAccessor{constructor: constructor}
})

// 4. 模型中使用
func (m *MyModel) DbType() model.DBType { return DBTypeRedis }
```

注册后，`DataAccessor` 会自动将请求路由到自定义实现。
