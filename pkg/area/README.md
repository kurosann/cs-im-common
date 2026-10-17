# 手机区号查询系统

这是一个完整的手机区号查询系统，可以根据手机号码匹配出对应的国家信息。

## 功能特性

- ✅ 支持通过手机号码查询国家信息（国家中文名、国家编码、电话区号）
- ✅ 支持通过区号直接查询国家信息
- ✅ 支持国家名称搜索功能
- ✅ 获取所有国家的完整列表
- ✅ 手机号码格式验证
- ✅ 自动清理手机号码中的非数字字符
- ✅ 支持多国家共享同一区号的情况

## 收录国家数量

共收录 **243** 个国家/地区的电话区号信息，包括：
- 主要国家和地区
- 海外领土和属地
- 特殊行政区

## 安装使用

```bash
# 在项目中导入包
import "vane-common/pkg/area"
```

## API 接口说明

### 1. 通过手机号码查询国家信息

```go
func GetCountryByPhone(phoneNumber string) []CountryInfo
```

**参数：**
- `phoneNumber`: 手机号码字符串（支持各种格式）

**返回：**
- `[]CountryInfo`: 匹配到的国家信息数组

**示例：**
```go
countries := area.GetCountryByPhone("+86 138 0013 8000")
// 返回: [{中国 CN 86}]

countries = area.GetCountryByPhone("+1 555 123 4567")
// 返回23个国家，因为+1区号被美国、加拿大等23个国家共享
```

### 2. 通过区号直接查询

```go
func GetCountryByCode(code int) []CountryInfo
```

**参数：**
- `code`: 电话区号（整数）

**返回：**
- `[]CountryInfo`: 匹配到的国家信息数组

**示例：**
```go
countries := area.GetCountryByCode(86)
// 返回: [{中国 CN 86}]

countries = area.GetCountryByCode(1)
// 返回23个国家信息
```

### 3. 搜索国家名称

```go
func SearchCountryByName(name string) []CountryInfo
```

**参数：**
- `name`: 国家名称关键词（支持模糊搜索）

**返回：**
- `[]CountryInfo`: 匹配到的国家信息数组

**示例：**
```go
countries := area.SearchCountryByName("中国")
// 返回: [{中国 CN 86} {香港（中国） HK 852} {澳门（中国） MO 853} {台湾（中国） TW 886}]

countries = area.SearchCountryByName("美国")
// 返回美国相关国家信息
```

### 4. 获取所有国家信息

```go
func GetAllCountries() []CountryInfo
```

**返回：**
- `[]CountryInfo`: 按区号排序的所有国家信息

### 5. 验证手机号码格式

```go
func IsValidPhoneFormat(phone string) bool
```

**参数：**
- `phone`: 手机号码字符串

**返回：**
- `bool`: 是否为有效格式（7-15位数字）

## CountryInfo 结构体

```go
type CountryInfo struct {
    CountryName string // 国家中文名称
    CountryCode string // 国家编码(ISO 3166-1 alpha-2)
    PhoneCode   int    // 电话区号
}
```

## 使用示例

```go
package main

import (
    "fmt"
    "vane-common/pkg/area"
)

func main() {
    // 1. 通过手机号码查询
    phoneNumber := "+86 138 0013 8000"
    countries := area.GetCountryByPhone(phoneNumber)
    
    if len(countries) > 0 {
        fmt.Printf("手机号码 %s 属于:\n", phoneNumber)
        for _, country := range countries {
            fmt.Printf("- %s (%s) 区号: +%d\n", 
                country.CountryName, 
                country.CountryCode, 
                country.PhoneCode)
        }
    }
    
    // 2. 直接通过区号查询
    chinaInfo := area.GetCountryByCode(86)
    fmt.Printf("区号+86对应的国家: %s\n", chinaInfo[0].CountryName)
    
    // 3. 搜索功能
    results := area.SearchCountryByName("日本")
    fmt.Printf("搜索'日本'找到 %d 个结果\n", len(results))
}
```

## 测试

运行单元测试：
```bash
go test -v
```

运行特定测试：
```bash
go test -v -run TestGetCountryByPhone
```

## 性能特点

- 🚀 快速查找：使用map数据结构，O(1)时间复杂度
- 📱 智能匹配：自动尝试不同长度的区号匹配
- 🔍 灵活搜索：支持模糊匹配国家名称
- 🌍 全面覆盖：涵盖243个国家和地区

## 注意事项

1. 区号1被23个国家共享（主要是北美地区）
2. 手机号码会自动清理非数字字符
3. 支持国际格式的号码输入（如 +86, 0086 等）
4. 返回结果可能包含多个国家（当多个地区使用相同区号时）

## 更新记录

- v1.0.0: 初始版本，支持基础查询功能
- 收录243个国家/地区电话区号
- 提供完整的API接口和测试用例