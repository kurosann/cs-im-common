package wallet

import (
	"encoding/hex"
	"math/rand"
	"os"
	"testing"
)

const pubKey = "049d76ef000d503864f95c2bc002b6761e15b23683d3b89ec7e8af0fa286c24222b6141011ed64e9c758e89332a7439361933c0f27945e88ecb6d1a4f5fea015a9"
const priKey = "307702010104205ef316a5cc0bfcab4d569fe58b2c35fe4d103c2676aeb1aab590a3621c45bb64a00a06082a8648ce3d030107a144034200049d76ef000d503864f95c2bc002b6761e15b23683d3b89ec7e8af0fa286c24222b6141011ed64e9c758e89332a7439361933c0f27945e88ecb6d1a4f5fea015a9"
const content = "Happyjava not only java"
const signature = "915080d548666ae302a0b188c0c6e0885b7fde7931d5066d5c61d5b3e58c132057395b439216156a9e3e109043da2f56dc3c51debbb09b13d0149bc3e7f48fd1"

func TestGenKeyPair(t *testing.T) {
	privateKey, publicKey := GenKeyPair()
	t.Log("pubKey:", publicKey)
	t.Log("priKey:", privateKey)
}

func TestBuildPublicKey(t *testing.T) {
	publicKey, e := BuildPublicKey(pubKey)
	if e != nil {
		t.Error(e)
		return
	}
	t.Log("publicKey:", publicKey)
}

func TestBuildPrivateKey(t *testing.T) {
	privateKey, e := BuildPrivateKey(priKey)
	if e != nil {
		t.Error(e)
		return
	}
	t.Log("privateKey:", privateKey)
}

func TestSign(t *testing.T) {
	signature, e := Sign(content, priKey)
	if e != nil {
		t.Error(e)
	} else {
		t.Log("signature:", signature)
	}
}

func TestVerifySign(t *testing.T) {
	verify := VerifySign(content, signature, pubKey)
	t.Log("verify:", verify)
}

func TestCreateECDHKey(t *testing.T) {
	// 生成Alice和Bob的密钥对
	privAlice, pubAlice := GenKeyPair()
	privBob, pubBob := GenKeyPair()

	t.Log("Alice公钥:", pubAlice)
	t.Log("Bob公钥:", pubBob)

	// Alice计算共享密钥（使用自己的私钥和Bob的公钥）
	keyAlice, _ := EcdsaCreateECDHKey(privAlice, pubBob)
	t.Log("Alice的共享密钥:", keyAlice)

	// Bob计算共享密钥（使用自己的私钥和Alice的公钥）
	keyBob, _ := EcdsaCreateECDHKey(privBob, pubAlice)
	t.Log("Bob的共享密钥:", keyBob)

	// 验证密钥一致性
	if keyAlice != keyBob {
		t.Errorf("密钥不一致: Alice=%s, Bob=%s", keyAlice, keyBob)
	} else {
		t.Log("ECDH密钥验证成功！")
	}

	// 使用共享密钥进行AES加密和解密
	sharedKeyBytes, _ := hex.DecodeString(keyAlice)
	data := []byte("Hello, ECDH with AES!")

	t.Log("AES Key:", hex.EncodeToString(sharedKeyBytes))
	// 加密
	encrypted, err := AesEncrypt(data, sharedKeyBytes)
	if err != nil {
		t.Fatal("加密失败:", err)
	}
	t.Log("密文:", encrypted)

	// 解密
	decrypted, err := AesDecrypt(encrypted, sharedKeyBytes)
	if err != nil {
		t.Fatal("解密失败:", err)
	}
	t.Log("解密后数据:", string(decrypted))

	// 验证解密结果
	if string(decrypted) != string(data) {
		t.Errorf("解密失败，原始数据: %s，解密后: %s", data, decrypted)
	} else {
		t.Log("AES加密解密验证成功！")
	}
}

func TestAESKey(t *testing.T) {
	// 示例1：生成随机AES-256密钥并加密
	key := GeneratePrintableKey(32)

	t.Log("随机AES-256密钥:", key)

	encry, err := AesEncrypt([]byte("isu"), []byte(key))
	if err != nil {
		return
	}
	decrypt, err := AesDecrypt(encry, []byte(key))
	if err != nil {
		return
	}
	t.Log("解密结果:", string(decrypt))

}

// GenerateAESKey 生成指定长度的 AES 密钥（16, 24 或 32 字节）
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

func GeneratePrintableKey(n int) string {
	b := make([]byte, n)
	rand.Read(b) // 填充随机性
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// 加密数据

func TestAESPEM(t *testing.T) {

	genKey := GeneratePrintableKey(32)
	t.Log(genKey)

	caPEM, err := os.ReadFile("./certs/ca.crt")
	if err != nil {
		t.Fatalf("Failed to read CA certificate: %v", err)
	}

	clientCertPEM, err := os.ReadFile("./certs/Tglink.crt")
	if err != nil {
		t.Fatalf("Failed to read client certificate: %v", err)
	}

	clientKeyPEM, err := os.ReadFile("./certs/Tglink.key")
	if err != nil {
		t.Fatalf("Failed to read client key: %v", err)
	}

	gkey := genKey
	for i := 0; i < 14; i++ {
		ky := GenerateAES256Key(gkey)
		gkey = string(ky)
	}

	crt1, err := AesEncrypt(clientCertPEM, []byte(gkey))
	if err != nil {
		t.Log(err.Error())
		return
	}
	t.Log(crt1)
	tgLinkKey1, err := AesEncrypt(clientKeyPEM, GenerateAES256Key(gkey))
	if err != nil {
		t.Log(err.Error())
		return
	}
	t.Log(tgLinkKey1)
	caCrt1, err := AesEncrypt(caPEM, GenerateAES256Key(gkey))
	if err != nil {
		t.Log(err.Error())
		return
	}
	t.Log(caCrt1)
}
