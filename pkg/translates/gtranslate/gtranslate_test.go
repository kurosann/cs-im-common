package gtranslate

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestTranslateWithFromTo(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	SetClient(&http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	})

	for i := 0; i < 4; i++ {
		for _, ta := range testingTable {
			resp, err := TranslateWithParams(context.Background(), ta.inText, TranslationParams{
				From:       ta.langFrom,
				To:         ta.langTo,
				Tries:      5,
				Delay:      time.Second,
				GoogleHost: "google.com",
			})
			if err != nil {
				t.Error(err, err.Error())
				t.Fail()
			}
			if resp != ta.outText {
				t.Error("translated text is not the expected", ta.outText, " != ", resp)
			}
			t.Logf("Translated %s from %s to %s: %s", ta.inText, ta.langFrom, ta.langTo, resp)
		}
	}
}
