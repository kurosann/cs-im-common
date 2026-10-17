package dec

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/shopspring/decimal"
)

func Int[T I](i T) decimal.Decimal {
	return decimal.NewFromInt(int64(i))
}
func Float[T F](f T) decimal.Decimal {
	return decimal.NewFromFloat(float64(f))
}
func String(s string) decimal.Decimal {
	if s == "" {
		return Zero
	}
	return decimal.RequireFromString(s)
}

var Zero = decimal.Zero

func AddMulti[T Num](i1 T, i2 ...T) decimal.Decimal {
	total := N(i1)
	for _, v := range i2 {
		total = total.Add(N(v))
	}
	return total
}
func SubMulti[T Num](i1 T, i2 ...T) decimal.Decimal {
	total := N(i1)
	for _, v := range i2 {
		total = total.Sub(N(v))
	}
	return total
}
func PowMulti[T Num, R Num](i1 T, i2 ...T) decimal.Decimal {
	total := N(i1)
	for _, v := range i2 {
		total = total.Pow(N(v))
	}
	return total
}
func MulMulti[T Num](i1 T, i2 ...T) decimal.Decimal {
	total := N(i1)
	for _, v := range i2 {
		total = total.Mul(N(v))
	}
	return total
}
func DivMulti[T Num](i1 T, i2 ...T) decimal.Decimal {
	total := N(i1)
	for _, v := range i2 {
		total = total.Div(N(v))
	}
	return total
}
func Add[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return N(i1).Add(N(i2))
}
func Sub[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return N(i1).Sub(N(i2))
}
func Pow[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return N(i1).Pow(N(i2))
}
func Mul[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return N(i1).Mul(N(i2))
}
func Div[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return N(i1).Div(N(i2))
}
func Max[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return decimal.Max(N(i1), N(i2))
}
func Min[T Num, R Num](i1 T, i2 R) decimal.Decimal {
	return decimal.Min(N(i1), N(i2))
}
func Mean(in []decimal.Decimal) decimal.Decimal {
	if len(in) == 0 {
		return Zero
	}
	return Div(Sum(in), len(in))
}

func Variance(in []decimal.Decimal, mean decimal.Decimal) decimal.Decimal {
	if len(in) == 0 {
		return Zero
	}
	total := decimal.Zero
	for _, v := range in {
		total = total.Add(v.Sub(mean).Pow(decimal.NewFromInt(2)))
	}
	return Div(total, decimal.NewFromInt(int64(len(in))))
}

func Sqrt(i1 decimal.Decimal) decimal.Decimal {
	if i1.LessThan(Zero) {
		return Zero
	}
	if i1.Equal(Zero) {
		return Zero
	}

	// 处理科学计数法形式
	coefficient := i1.Coefficient()
	exponent := i1.Exponent()

	// 调整奇偶指数
	if exponent%2 != 0 {
		coefficient = coefficient.Mul(coefficient, big.NewInt(10))
		exponent--
	}

	// 构建初始猜测值：sqrt(coefficient) * 10^(exponent/2)
	sqrtCoeff := sqrtBigInt(coefficient) // 自定义大整数平方根算法
	x := decimal.NewFromBigInt(sqrtCoeff, exponent/2)

	// 牛顿迭代法优化
	precision := int32(16) // 提高迭代精度
	maxIterations := 30    // 增加最大迭代次数

	for i := 0; i < maxIterations; i++ {
		lastX := x
		// 迭代公式: x = (x + n/x)/2
		x = x.Add(i1.Div(x)).Div(N(2)).Round(precision)
		// 收敛判断（使用更严格的终止条件）
		if diff := x.Sub(lastX).Abs(); diff.LessThan(N(1e-20)) {
			break
		}
	}
	return x.Round(8) // 最终保留8位小数
}

// 自定义大整数平方根算法
func sqrtBigInt(n *big.Int) *big.Int {
	if n.Sign() < 0 {
		return big.NewInt(0)
	}

	x := new(big.Int)
	x.Set(n)
	y := new(big.Int)
	y.Add(x, big.NewInt(1)).Div(y, big.NewInt(2))

	for x.Cmp(y) > 0 {
		x.Set(y)
		y.Add(x, new(big.Int).Div(n, x)).Div(y, big.NewInt(2))
	}
	return x
}

// Eq 等于
func Eq[T Num, R Num](i1 T, i2 R) bool {
	return N(i1).Equal(N(i2))
}

// GT 大于
func GT[T Num, R Num](i1 T, i2 R) bool {
	return N(i1).GreaterThan(N(i2))
}

// GTE 大于等于
func GTE[T Num, R Num](i1 T, i2 R) bool {
	return N(i1).GreaterThanOrEqual(N(i2))
}

// LT 小于
func LT[T Num, R Num](i1 T, i2 R) bool {
	return N(i1).LessThan(N(i2))
}

// LTE 小于等于
func LTE[T Num, R Num](i1 T, i2 R) bool {
	return N(i1).LessThanOrEqual(N(i2))
}

func Sum(in []decimal.Decimal) decimal.Decimal {
	total := N(0)
	for _, a := range in {
		total = Add(total, a)
	}
	return total
}
func Avg(in []decimal.Decimal) decimal.Decimal {
	if len(in) == 0 {
		return Sum(in)
	}
	return Div(Sum(in), len(in))
}

func N(e any) decimal.Decimal {
	switch t := e.(type) {
	case int64:
		return Int(t)
	case int32:
		return Int(t)
	case int8:
		return Int(t)
	case int16:
		return Int(t)
	case byte:
		return Int(t)
	case int:
		return Int(t)
	case float64:
		return Float(t)
	case float32:
		return Float(t)
	case string:
		return String(t)
	case decimal.Decimal:
		return t
	default:
		ofA := reflect.ValueOf(e)
		if !ofA.IsValid() || ofA.IsNil() || ofA.IsZero() {
			return Zero
		}
		if s, ok := e.(interface{ String() string }); ok {
			return String(s.String())
		}
		panic(fmt.Sprintf("decimal type is not support: %v", t))
	}
}

// RoundSignificant 保留5位小数
func RoundSignificant(d decimal.Decimal) decimal.Decimal {
	// 如果数字大于等于1，直接保留5位小数
	if GTE(d, 1) {
		return d.Round(5)
	}

	// 对于小于1的数字，从第一个非零数字开始计算
	str := d.String()

	foundDecimal := false
	zeroCount := 0

	for _, c := range str {
		if c == '.' {
			foundDecimal = true
			continue
		}
		if foundDecimal {
			if c == '0' {
				zeroCount++
			} else {
				zeroCount++ // 包括第一个非零数字
				break
			}
		}
	}

	// 计算需要保留的小数位数
	precision := 5 + zeroCount
	//logger.Warnf("计算需要保留的小数位数%d,保留后：%v", precision, d.Round(int32(precision)))
	return d.Round(int32(precision))
}
