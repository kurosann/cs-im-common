package gtranslate

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/text/language"
)

var GoogleHost = "google.cn"

// TranslationParams is a util struct to pass as parameter to indicate how to translate
type TranslationParams struct {
	Client     *http.Client
	From       string
	To         string
	Tries      int
	Delay      time.Duration
	GoogleHost string
}

// Translate translate a text using native tags offer by go language
func Translate(ctx context.Context, text string, from language.Tag, to language.Tag, googleHost ...string) (string, error) {
	if len(googleHost) != 0 && googleHost[0] != "" {
		GoogleHost = googleHost[0]
	}
	translated, err := translate(ctx, text, from.String(), to.String(), false, 2, 0)
	if err != nil {
		return "", err
	}

	return translated, nil
}

// TranslateWithParams translate a text with simple params as string
func TranslateWithParams(ctx context.Context, text string, params TranslationParams) (string, error) {
	if params.GoogleHost == "" {
		GoogleHost = "google.com"
	} else {
		GoogleHost = params.GoogleHost
	}

	// 使用自定义客户端
	if params.Client != nil {
		translated, err := translateWithClient(ctx, translateParam{
			From:             params.From,
			To:               params.To,
			Text:             text,
			WithVerification: true,
			Tries:            params.Tries,
			Delay:            params.Delay,
		}, params.Client)
		if err != nil {
			return "", err
		}
		return translated, nil
	}

	// 使用默认客户端
	translated, err := translate(ctx, text, params.From, params.To, true, params.Tries, params.Delay)
	if err != nil {
		return "", err
	}
	return translated, nil
}
