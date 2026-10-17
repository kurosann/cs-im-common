package jsencry

import (
	"fmt"
	"math/rand"
	"strings"
)

func GenerateEncryCode() (string, string) {
	formulas := make([]string, 0)
	for i := 0; i < 4; i++ {
		formulas = append(formulas, fmt.Sprintf("(%s+%d*1)", RandExprXY("x", "y"), i+1))
	}
	// 让数据的每个分割符都是随机的
	formula := strings.Join(formulas, RandOpList([]string{"+", "-", "*"}))
	opStr := RandOpList([]string{"+", "*"})
	sk := rand.Intn(100) + 1
	someNumber := rand.Intn(3) + 3
	formula = fmt.Sprintf("(%s * 3 + 1) %% %d +1", formula, someNumber)
	// 执行 JS 脚本
	code1 := fmt.Sprintf(`
		function GenFormula(x,y,r) {
			let sk = %d;
			let G = %s;
			let someNumber = %d;
			if (Number.isNaN(G) === true) {
			 	G = sk %s someNumber;
			}
			if (G<0) {
			  	G = -G;
			}
			G = G + someNumber;
			let PK = sk * G;
			let R = r * G;
			let c = PK * R;
			let z = r + c * sk;
			return [z,R,G]
		}
		GenFormula(x, y, r);
	`, sk, formula, someNumber, opStr)

	// G 作为随机计算公式
	// s、k 作为已知
	// 前端拿到 sk, 随机 r
	// 计算 c 、z、R
	// 每次请求返回 G值, R 、z
	// 后端校验 z*G?==R+c*P

	code2 := fmt.Sprintf(`
		function VerifyFormula(G,R,z) {
			let a = %d;
			let PK = a*G;
			let c = PK*R;
			return z*G == R+c*PK
		}
		VerifyFormula(G,R,z);
	`, sk)

	return code1, code2
}

func VerifyFormula(code string, G, R, z int) bool {
	runner := New()

	runner.Set("G", G)
	runner.Set("R", R)
	runner.Set("z", z)
	result, err := runner.Run(code)
	if err != nil {
		fmt.Println(result)
		return false
	}
	return result.(bool)
}
