package dec

import "github.com/shopspring/decimal"

type I interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~byte
}
type F interface {
	~float32 | ~float64
}
type Num interface {
	I | F | ~string | decimal.Decimal
}
