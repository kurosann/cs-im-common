package authx

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestJwt(t *testing.T) {
	// 生成 token
	token, err := GenerateJWT("user123", "11111")
	if err != nil {
		fmt.Println("Error generating token:", err)
		return
	}
	fmt.Println("JWT Token:", token)

	// 验证 token
	isValid, err := VerifyJWT(token, "11111")
	if err != nil {
		fmt.Println("Verification error:", err)
		return
	}
	fmt.Println("Token is valid:", isValid)

	// 模拟过期 token（通过修改生成时间）
	// 这里仅为演示，实际过期需要时间流逝
	claims := jwt.MapClaims{
		"sub": "user123",
		"exp": time.Now().Add(time.Second * -1).Unix(), // 设置为已过期
		"iat": time.Now().Unix(),
	}
	tokenExpired := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStringExpired, _ := tokenExpired.SignedString([]byte("11111"))

	isValid, err = VerifyJWT(tokenStringExpired, "11111")
	if err != nil {
		fmt.Println("Verification error for expired token:", err)
	} else {
		fmt.Println("Expired token is valid:", isValid)
	}
}
