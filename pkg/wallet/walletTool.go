package wallet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// GenerateAES256Key 使用SHA256哈希生成32字节AES密钥
func GenerateAES256Key(input string) []byte {
	hash := sha256.Sum256([]byte(input))
	return hash[:]
}

// GenerateAES128Key 使用MD5哈希生成16字节AES密钥
func GenerateAES128Key(input string) []byte {
	hash := md5.Sum([]byte(input))
	return hash[:]
}

func CreateKey() (privKey, pubKey string) {
	aliceKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	alicePubkey := aliceKey.PublicKey()

	// 秘钥哈希(Alice)  a74e7949e71ead5f3bd4de031e2ad45c3f5b80b48ccf50e50eb86f4bdb025c3a
	return hex.EncodeToString(aliceKey.Bytes()), hex.EncodeToString(alicePubkey.Bytes())
}

func CreateECDHKey(privKey, pubKey []byte) (k string, err error) {
	priKey, err := ecdh.P256().NewPrivateKey(privKey)
	if err != nil {
		return "", err
	}
	pukey, err := ecdh.P256().NewPublicKey(pubKey)
	if err != nil {
		return "", err
	}
	shared, _ := priKey.ECDH(pukey)
	aliceShared := sha256.Sum256(shared)
	return hex.EncodeToString(aliceShared[:]), err

}

// AesEncrypt 加密
func AesEncrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// 填充 plaintext 到块大小
	plaintext = pad(plaintext, block.BlockSize())

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	// 生成随机 IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	return hex.EncodeToString(ciphertext), nil
}

// AesDecrypt 解密
func AesDecrypt(ciphertextHex string, key []byte) ([]byte, error) {
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// 去填充
	return unpad(ciphertext)
}

// 填充
func pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

// 去填充
func unpad(src []byte) ([]byte, error) {
	length := len(src)
	unpadding := int(src[length-1])
	if unpadding > length {
		return nil, fmt.Errorf("unpadding error")
	}
	return src[:(length - unpadding)], nil
}

func StrSha256(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateRandomAESKey 生成指定长度的随机AES密钥（推荐生产使用）
// 支持 AES-128/192/256（16/24/32字节），完全兼容现有的 AES-CBC 加解密函数
// 使用 crypto/rand 确保密码学安全随机性
func GenerateRandomAESKey(keyLen int) ([]byte, error) {
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid AES key length %d, must be 16, 24 or 32", keyLen)
	}

	key := make([]byte, keyLen)
	fmt.Println("key:", key)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateRandomAES256Key 生成随机32字节AES-256密钥（最常用）
func GenerateRandomAES256Key() ([]byte, error) {
	return GenerateRandomAESKey(32)
}

// GenerateRandomAES128Key 生成随机16字节AES-128密钥
func GenerateRandomAES128Key() ([]byte, error) {
	return GenerateRandomAESKey(16)
}
