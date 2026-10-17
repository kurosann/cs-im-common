package gtranslate

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/logger"

	"go.uber.org/zap"
)

var (
	ttkCache   = "0"
	ttkCacheMu sync.RWMutex
	ttkRe      = regexp.MustCompile(`tkk:\s?'(.+?)'`)
)

func sM(text string, ttk string) string {
	if ttk == "" {
		ttk = "0"
	}

	parts := strings.Split(ttk, ".")
	var bVal int64
	if len(parts) > 0 {
		bVal, _ = strconv.ParseInt(parts[0], 10, 64)
	}

	var d1 int64
	if len(parts) > 1 {
		d1, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	e := []byte(text)

	a := bVal
	for i := 0; i < len(e); i++ {
		a += int64(e[i])
		a = xr(a, "+-a^+6")
	}
	a = xr(a, "+-3^+b+-f")
	a = int64(int32(a) ^ int32(d1))

	if a < 0 {
		a = int64(uint32(a))
	}
	a %= 1000000

	return fmt.Sprintf("&tk=%d.%d", a, a^bVal)
}

func xr(a int64, b string) int64 {
	for c := 0; c < len(b)-2; c += 3 {
		d := b[c+2]
		var dd int64
		if d >= 'a' && d <= 'z' {
			dd = int64(d) - 87
		} else {
			dd = int64(d - '0')
		}

		if b[c+1] == '+' {
			dd = int64(uint32(a) >> uint(dd))
		} else {
			dd = int64(uint32(a) << uint(dd))
		}

		if b[c] == '+' {
			a = int64(uint32(a) + uint32(dd))
		} else {
			a = int64(int32(a) ^ int32(dd))
		}
	}
	return a
}

func updateTTK(ctx context.Context) (string, error) {
	return updateTTKWithClient(ctx, nil)
}

func updateTTKWithClient(ctx context.Context, client *http.Client) (string, error) {
	t := time.Now().UnixNano() / 3600000
	now := int64(math.Floor(float64(t)))

	httpClient := client
	if httpClient == nil {
		httpClient = globalHTTPClient
	}

	ttkCacheMu.RLock()
	cached := ttkCache
	ttkCacheMu.RUnlock()

	cachedVal, err := strconv.ParseFloat(cached, 64)
	if err == nil && int64(cachedVal) == now {
		logger.Debug(ctx, debugGTranslateHTTPLogPrefix+" token cache hit",
			zap.String("host", "translate."+GoogleHost),
			zap.Any("httpClient", DescribeHTTPClient(httpClient)),
			zap.Bool("customClient", client != nil),
		)
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://translate.%s", GoogleHost), nil)
	if err != nil {
		return cached, err
	}

	logHTTPClientRequest(ctx, debugGTranslateHTTPLogPrefix+" token request", httpClient, req,
		zap.Bool("customClient", client != nil),
	)

	resp, err := httpClient.Do(req)
	if err != nil {
		logHTTPClientRequestError(ctx, debugGTranslateHTTPLogPrefix+" token request failed", httpClient, req, err,
			zap.Bool("customClient", client != nil),
		)
		return cached, err
	}
	defer resp.Body.Close()

	logHTTPClientResponse(ctx, debugGTranslateHTTPLogPrefix+" token response", httpClient, req, resp,
		zap.Bool("customClient", client != nil),
	)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return cached, err
	}

	matches := ttkRe.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		newTTK := matches[1]
		ttkCacheMu.Lock()
		ttkCache = newTTK
		ttkCacheMu.Unlock()
		return newTTK, nil
	}

	return cached, nil
}

func get(ctx context.Context, text string) string {
	return getWithClient(ctx, text, nil)
}

func getWithClient(ctx context.Context, text string, client *http.Client) string {
	ttkStr, err := updateTTKWithClient(ctx, client)
	if err != nil {
		ttkCacheMu.RLock()
		ttkStr = ttkCache
		ttkCacheMu.RUnlock()
	}

	tk := sM(text, ttkStr)
	return strings.TrimPrefix(tk, "&tk=")
}
