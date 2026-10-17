package gtranslate

import (
	"context"
	"net/http"
	"time"
)

// translateParam 翻译参数
type translateParam struct {
	Text             string        `json:"text"`             // 待翻译文本
	From             string        `json:"from"`             // 源语言
	To               string        `json:"to"`               // 目标语言
	WithVerification bool          `json:"withVerification"` // 是否开启验证
	Tries            int           `json:"tries"`            // 重试次数
	Delay            time.Duration `json:"delay"`            // 重试延迟时间
}

func translateWithClient(ctx context.Context, param translateParam, client *http.Client) (string, error) {
	from, to := normalizeLang(param.From, param.To, param.WithVerification)
	return doLightTranslate(ctx, client, param.Text, from, to, param.Tries, param.Delay)
}
