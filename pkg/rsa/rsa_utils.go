package rsa

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

// GenRsaKey 生成RSA密钥对（私钥和公钥）
func GenRsaKey(bits int) (privatePEM, publicPEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return
	}

	// 私钥编码为PEM格式
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privateBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateDER,
	}
	privatePEM = string(pem.EncodeToMemory(&privateBlock))

	// 公钥编码为PEM格式
	publicKey := &privateKey.PublicKey
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return
	}
	publicBlock := pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicDER,
	}
	publicPEM = string(pem.EncodeToMemory(&publicBlock))

	return
}

// Encrypt 加密数据（使用公钥）
func Encrypt(publicPEM, data string) (string, error) {
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		return "", errors.New("invalid public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaPub := pub.(*rsa.PublicKey)

	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, []byte(data))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据（使用私钥）
func Decrypt(privatePEM, encryptedData string) (string, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return "", errors.New("invalid private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	encrypted, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}

	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encrypted)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Sign 签名数据（使用私钥）
func Sign(privatePEM []byte, data []byte) ([]byte, error) {
	block, _ := pem.Decode(privatePEM)
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
}

// Verify 验证签名（使用公钥）
func Verify(publicPEM []byte, data, signature []byte) error {
	block, _ := pem.Decode(publicPEM)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	rsaPub := pub.(*rsa.PublicKey)

	hash := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hash[:], signature)
}
