package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"math/big"
)

// GenKeyPair 生成密钥对（无需修改）
func GenKeyPair() (privateKey string, publicKey string) {
	priKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return "", ""
	}

	ecPrivateKey, e := x509.MarshalECPrivateKey(priKey)
	privateKey = hex.EncodeToString(ecPrivateKey)

	pubBytes := elliptic.Marshal(elliptic.P256(), priKey.PublicKey.X, priKey.PublicKey.Y)
	publicKey = hex.EncodeToString(pubBytes)

	return privateKey, publicKey
}

// BuildPrivateKey 构建私钥（无需修改）
func BuildPrivateKey(privateKeyStr string) (*ecdsa.PrivateKey, error) {
	bytes, e := hex.DecodeString(privateKeyStr)
	if e != nil {
		return nil, e
	}
	return x509.ParseECPrivateKey(bytes)
}

// BuildPublicKey 构建公钥（修正方法）
func BuildPublicKey(publicKeyStr string) (*ecdsa.PublicKey, error) {
	pubBytes, e := hex.DecodeString(publicKeyStr)
	if e != nil {
		return nil, e
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), pubBytes)
	if x == nil || y == nil {
		return nil, errors.New("invalid public key format")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// Sign 签名函数（关键修改）
func Sign(content string, privateKeyStr string) (string, error) {
	priKey, e := BuildPrivateKey(privateKeyStr)
	if e != nil {
		return "", e
	}

	hash := sha256.Sum256([]byte(content))
	r, s, e := ecdsa.Sign(rand.Reader, priKey, hash[:])
	if e != nil {
		return "", e
	}

	// 将 r 和 s 转为固定长度字节数组（32字节）
	rBytes := pade(r.Bytes(), 32)
	sBytes := pade(s.Bytes(), 32)

	signature := append(rBytes, sBytes...)
	return hex.EncodeToString(signature), nil
}

// 5. 验证函数（关键修改）
func VerifySign(content string, signature string, publicKeyStr string) bool {
	sigBytes, e := hex.DecodeString(signature)
	if e != nil {
		return false
	}

	pubKey, e := BuildPublicKey(publicKeyStr)
	if e != nil {
		return false
	}

	hash := sha256.Sum256([]byte(content))
	r := big.Int{}
	s := big.Int{}

	// 解析 r 和 s（各32字节）
	r.SetBytes(sigBytes[:32])
	s.SetBytes(sigBytes[32:])

	return ecdsa.Verify(pubKey, hash[:], &r, &s)
}

// 辅助函数：补零到指定长度
func pade(b []byte, size int) []byte {
	if len(b) > size {
		return b[len(b)-size:]
	}
	padding := make([]byte, size-len(b))
	return append(padding, b...)
}

// EcdsaCreateECDHKey 通过ECDH算法生成共享密钥
func EcdsaCreateECDHKey(privateKeyStr string, peerPublicKeyStr string) (string, error) {
	// 1. 解析本地私钥
	privateKey, err := BuildPrivateKey(privateKeyStr)
	if err != nil {
		return "", err
	}

	// 2. 解析对方公钥
	peerPublicKey, err := BuildPublicKey(peerPublicKeyStr)
	if err != nil {
		return "", err
	}

	// 3. 执行ECDH计算：d * peerPubKey = (x, y)
	x, y := peerPublicKey.Curve.ScalarMult(
		peerPublicKey.X,
		peerPublicKey.Y,
		privateKey.D.Bytes(),
	)
	if x == nil || y == nil {
		return "", errors.New("无效的ECDH计算结果")
	}

	// 4. 提取x坐标并进行哈希处理生成共享密钥（使用SHA256）
	hash := sha256.New()
	hash.Write(x.Bytes())
	sharedKey := hash.Sum(nil)

	// 5. 返回十六进制编码的共享密钥
	return hex.EncodeToString(sharedKey), nil
}
