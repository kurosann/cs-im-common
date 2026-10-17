package rsa

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRsa(t *testing.T) {
	// 生成密钥对
	private, public, _ := GenRsaKey(2048)
	t.Log(private)
	t.Log(public)
	// 加密解密
	encrypted, _ := Encrypt(public, "tlg pilot server RSA")
	decrypted, _ := Decrypt(private, encrypted)
	fmt.Println(decrypted)
	// 签名验证
	signature, _ := Sign([]byte(private), []byte("data"))
	Verify([]byte(public), []byte("data"), signature)

}

func TestEncryptDecrypt(t *testing.T) {
	// 生成测试密钥对
	private, public, err := GenRsaKey(2048)
	if err != nil {
		t.Fatal("密钥生成失败:", err)
	}

	testCases := []string{
		"Hello RSA",
		"中文测试",
		"1234567890",
		"",
	}

	for _, data := range testCases {
		t.Run(data, func(t *testing.T) {
			// 加密过程
			encrypted, err := Encrypt(public, data)
			assert.NoError(t, err, "加密应成功")
			assert.NotEmpty(t, encrypted, "加密结果不应为空")

			// 解密过程
			decrypted, err := Decrypt(private, encrypted)
			assert.NoError(t, err, "解密应成功")

			// 验证结果
			assert.Equal(t, data, decrypted, "解密结果应与原始数据一致")
		})
	}
}
