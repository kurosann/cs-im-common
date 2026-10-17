package deeplx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/avast/retry-go"
	"github.com/tidwall/gjson"
)

var (
	wg             sync.WaitGroup
	translateMutex sync.Mutex // 添加互斥锁
	targetUrls     = make([]string, 0)
	urls           = []string{
		"https://deeplx.mingming.dev:443/translate",
		"https://deeplx-2dn.pages.dev:443/translate",
		"https://deeplx-2dn.pages.dev:8443/translate",
		"https://deeplx.mingming.dev:2083/translate",
		"https://deeplx.mingming.dev:2053/translate",
		"https://deeplx.mingming.dev:8443/translate",
	}
)

type Response struct {
	Code int64  `json:"code"`
	Data string `json:"data"`
	Msg  string `json:"msg"`
}

func Translate(ctx context.Context, text, sourceLang, targetLang string, cli ...*http.Client) *Response {
	// 获取锁
	translateMutex.Lock()
	defer translateMutex.Unlock() // 确保函数退出时释放锁
	return TranslateWithProxyUrl(ctx, text, sourceLang, targetLang, cli...)
}

func TranslateWithProxyUrl(ctx context.Context, text, sourceLang, targetLang string, cli ...*http.Client) (resp *Response) {

	// 加个捕获异常
	defer func() {
		if err := recover(); err != nil {
			resp = &Response{
				Code: 500,
				Msg:  "Translate Failed",
			}
			return
		}
	}()

	if len(text) == 0 {
		return &Response{
			Code: 500,
			Msg:  "No Translate Text Found",
		}
	}

	if len(sourceLang) == 0 {
		lang := whatlanggo.DetectLang(text)
		sourceLang = strings.ToUpper(lang.Iso6391())
	}

	if len(targetLang) == 0 {
		targetLang = "EN"
	}

	requestParams := bytes.Buffer{}
	requestParams.WriteString(`{"text":"` + text + `","source_lang":"` + sourceLang + `","target_lang":"` + targetLang + `"}`)

	httpClient := &http.Client{
		Timeout: 20 * time.Second,
	}
	if len(cli) == 0 || cli[0] == nil {
		transport := &http.Transport{}
		transport.Proxy = http.ProxyFromEnvironment
		httpClient.Transport = transport
	} else {
		httpClient = cli[0]
	}
	var body []byte
	err := retry.Do(
		func() error {
			// 创建请求对象
			req, err := http.NewRequestWithContext(ctx, "POST", fetchUri(), strings.NewReader(requestParams.String()))
			if err != nil {
				body = []byte(`{"code":500, "message": "` + err.Error() + `"}`)
				return err
			}
			// 设置请求头
			req.Header.Set("Content-Type", "application/json")

			// 使用 Do 发送请求
			response, err := httpClient.Do(req)

			if err != nil {
				if errors.Is(err, context.Canceled) {
					body = []byte(`{"code":500, "message": "` + "context canceled" + `"}`)
					return nil
				}
				body = []byte(`{"code":500, "message": "` + err.Error() + `"}`)
				return err
			}

			defer func() {
				_ = response.Body.Close()
			}()

			body, err = io.ReadAll(response.Body)
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
	)

	if err != nil {
		return TranslateByDeeplx(text, sourceLang, targetLang, getProxyUrl())
	}

	return &Response{
		Code: gjson.Get(string(body), "code").Int(),
		Data: gjson.Get(string(body), "data").String(),
		Msg:  gjson.Get(string(body), "message").String(),
	}
}

func TranslateByDeeplx(text, sourceLang, targetLang, proxyUrl string) *Response {
	result, err := TranslateByDeepLX(sourceLang, targetLang, text, "", proxyUrl, "")
	if err != nil {
		return &Response{
			Code: 500,
			Msg:  err.Error(),
		}
	}

	return &Response{
		Code: int64(result.Code),
		Data: result.Data,
		Msg:  result.Message,
	}
}
