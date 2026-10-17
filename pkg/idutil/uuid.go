package idutil

import (
	"encoding/base64"

	"github.com/google/uuid"
)

func GenShortId() string {
	return GenId(8)
}

func GenId(num int) string {
	bin, _ := uuid.New().MarshalBinary()
	if len(bin) < num {
		num = len(bin)
	}
	return base64.RawURLEncoding.EncodeToString(bin[:num])
}

func GenUUID() string {
	return uuid.New().String()
}
