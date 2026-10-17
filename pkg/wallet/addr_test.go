package wallet

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestTronAddr(t *testing.T) {
	for i := 0; i < 10; i++ {
		b5, pk := generateKeyPair()
		fmt.Printf("%s\n%s\n", b5, pk)
	}
}

func TestECDH(t *testing.T) {
	prv1, pu1 := CreateKey()
	prv2, pu2 := CreateKey()
	// f24f3b6b3402f170cae57ed9779692f79cdb9c3eb2739bf61be6e1911f2708fc
	// 0407da7d724d0ce1c2d4338563e98b3fab7969a2eaa37db9b85363e5f4ee3d0d4cac252811cd7cbe2a130572ffa11794f02b1674d6a42ecd8f015c954401675694
	decodeprv1, err := hex.DecodeString(prv1)
	if err != nil {
		return
	}
	decodepu1, err := hex.DecodeString(pu1)
	if err != nil {
		return
	}
	decodeprv2, err := hex.DecodeString(prv2)
	if err != nil {
		return
	}
	decodepu2, err := hex.DecodeString(pu2)
	if err != nil {
		return
	}
	b1, err := CreateECDHKey(decodeprv1, decodepu2)
	if err != nil {
		fmt.Println(err)
	}
	b2, err := CreateECDHKey(decodeprv2, decodepu1)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("%s\n%s\n", b1, b2)
}

func TestBase64(t *testing.T) {
	// 要转换的十六进制字符串
	data := "0407da7d724d0ce1c2d4338563e98b3fab7969a2eaa37db9b85363e5f4ee3d0d4cac252811cd7cbe2a130572ffa11794f02b1674d6a42ecd8f015c954401675694"
	dataBytes, err := hex.DecodeString(data)
	if err != nil {
		t.Log(err)
		return
	}

	// 将十六进制字符串转换为Base64编码
	base64Encoded := base64.StdEncoding.EncodeToString(dataBytes)
	fmt.Printf("Base64 Encoded: %s\n", base64Encoded)
}
