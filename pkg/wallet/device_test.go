package wallet

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestDevice(t *testing.T) {
	// 设备信息
	device := "windows11 E-A-B-C-D"

	// 对设备信息进行MD5哈希
	deviceHash := md5.Sum([]byte(device))
	deviceHashString := hex.EncodeToString(deviceHash[:])

	//privAlice, pubAlice, _ := GenKeyPair()
	//t.Log("Alice公钥:", pubAlice)

	// 客户端私钥公钥
	privCKey, pubCKey := GenKeyPair()

	// 服务端私钥 公钥
	privSysKey, pubSysKey := GenKeyPair()

	// 服务端 公钥 --> 客户端
	// 生成共享密钥
	ecdhKey, err := EcdsaCreateECDHKey(privCKey, pubSysKey)
	if err != nil {
		t.Log(err)
		return
	}

	//ecdhKey
	decodeEcdhKey, err := hex.DecodeString(ecdhKey)
	if err != nil {
		t.Log(err)
		return
	}
	// 加密数据传输
	secretText, err := AesEncrypt([]byte(deviceHashString), decodeEcdhKey)
	// 密文 + 客户端公钥 --> 服务端
	ecdhSysKey, err := EcdsaCreateECDHKey(privSysKey, pubCKey)
	if err != nil {
		t.Log(err)
		return
	}
	decodeString, err := hex.DecodeString(ecdhSysKey)
	if err != nil {
		t.Log(err)
		return
	}
	// 解密
	secretTextByte, err := AesDecrypt(secretText, decodeString)
	if err != nil {
		t.Log(err)
		return
	}
	if string(secretTextByte) == deviceHashString {
		t.Log("success")
	}
	//
}

func TestMD5(t *testing.T) {

	device := "04203009c4a5d7f22dd252ba31651c22396ebafa1ea97e906add904bd81077a8cba8cb15cf207e940c605c73d0ef43c2e85c3b0bee66cf840c013788447f21106e"
	deviceHash := md5.Sum([]byte(device))
	deviceHashString := hex.EncodeToString(deviceHash[:])
	fmt.Println(deviceHashString)

	h := sha256.New()
	h.Write([]byte(device))
	sha256Hash := h.Sum(nil)
	sha256HashString := hex.EncodeToString(sha256Hash)
	fmt.Println(sha256HashString)
}

// 在device_test.go中添加以下测试函数：
func TestAesEncryptDecrypt(t *testing.T) {
	// 测试数据
	plaintext := []byte("04203009c4a5d7f22dd252ba31651c22396ebafa1ea97e906add904bd81077a8cba8cb15cf207e940c605c73d0ef43c2e85c3b0bee66cf840c013788447f21106e")
	key := []byte("0123456789abcdef0123456789abcdef") // 32字节固定密钥

	// 加密
	ciphertextHex, err := AesEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	t.Logf("加密结果: %s", ciphertextHex)
	// 解密
	decrypted, err := AesDecrypt(ciphertextHex, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	// 验证结果
	if string(decrypted) != string(plaintext) {
		t.Errorf("解密数据不匹配，原始: %s，解密后: %s", plaintext, decrypted)
	}

	t.Log("AES加解密测试通过")
}

func TestDeAES(t *testing.T) {
	secretTextByte, err := AesDecrypt("46b0e855f549933c3375ddfa0ed3b219c5de41cba87b10f21b6534e8dfac6229", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Log(err)
		return
	}
	t.Log(string(secretTextByte))
}
