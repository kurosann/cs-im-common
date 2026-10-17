package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTransText(t *testing.T) {
	InitI18n(map[TransCode]map[Language]string{
		"network.bizOutlet.outletName": {
			"en": "default",
		},
	})
	text := GetTransText("network.bizOutlet.outletName", "en")
	assert.Equal(t, text, "default")
}
