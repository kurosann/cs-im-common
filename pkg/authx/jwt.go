package authx

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(userID string, secret string) (string, error) {
	// 定义密钥
	secretKey := []byte(secret)

	// 创建 claims
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour * 24).UnixMilli(), // 过期时间
		"iat": time.Now().UnixMilli(),                     // 发行时间
	}

	// 创建 token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名 token
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// VerifyJWT 验证 JWT token
func VerifyJWT(tokenString string, secret string) (bool, error) {
	// 定义密钥
	secretKey := []byte(secret)

	// 解析 token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to parse token: %v", err)
	}

	// 检查 token 是否有效
	if !token.Valid {
		return false, fmt.Errorf("invalid token")
	}

	// 提取 claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("invalid claims format")
	}

	// 检查过期时间
	exp, ok := claims["exp"].(float64)
	if !ok {
		return false, fmt.Errorf("expiration time not found or invalid")
	}

	// 转换为时间
	if time.Now().UnixMilli() > int64(exp) {
		return false, fmt.Errorf("token has expired")
	}

	return true, nil
}

func GenSelfJWT(data interface{}, secret string) (string, error) {
	// 定义密钥
	secretKey := []byte(secret)
	byts, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sub := base64.StdEncoding.EncodeToString(byts)

	// 创建 claims
	claims := jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(time.Hour * 48).UnixMilli(), // 过期时间
		"iat": time.Now().UnixMilli(),                     // 发行时间
	}

	// 创建 token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名 token
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
func DecodeSelfJWT(tokenString string, secret string) ([]byte, error) {
	// 定义密钥
	secretKey := []byte(secret)

	// 解析 token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	// 检查 token 是否有效
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// 提取 claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	deSub, err := base64.StdEncoding.DecodeString(claims["sub"].(string))
	if err != nil {
		return nil, err
	}

	return deSub, nil
}
