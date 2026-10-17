package jsencry

import (
	"fmt"
	"math/rand"
	"time"
)

var ops = []string{"+", "-", "*", "%"}

// RandOp 返回一个随机运算符
func RandOp() string {
	rand.Seed(time.Now().UnixNano())
	return ops[rand.Intn(len(ops))]
}

// RandOpList 返回一个随机运算符
func RandOpList(ps []string) string {
	rand.Seed(time.Now().UnixNano())
	return ps[rand.Intn(len(ps))]
}

// RandExpr 生成一个简单的随机算式 (a op b)
func RandExpr(min, max int) string {
	rand.Seed(time.Now().UnixNano())
	a := rand.Intn(max-min+1) + min
	b := rand.Intn(max-min+1) + min
	op := RandOp()
	return fmt.Sprintf("%d %s %d", a, op, b)
}

// RandExprXY 生成一个简单的随机算式 (a op b)
func RandExprXY(x, y string) string {
	rand.Seed(time.Now().UnixNano())
	op := RandOp()
	return fmt.Sprintf("%s %s %s", x, op, y)
}
