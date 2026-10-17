package gtranslate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/text/language"
)

// Global HTTP client with proxy support
var globalHTTPClient *http.Client

var outTime = 30 * time.Second

func init() {
	// Initialize with default client
	globalHTTPClient = &http.Client{
		Timeout: outTime,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       outTime,
			DisableKeepAlives:     true,
			ExpectContinueTimeout: outTime,
			Proxy:                 http.ProxyFromEnvironment,
		},
	}
}

const (
	defaultNumberOfRetries = 2
	// Lightweight Google Translate endpoint used by ft-link:
	// single request, no TKK token scrape, only translation text (dt=t).
	googleTranslateEndpoint = "https://translate.googleapis.com/translate_a/single"
)

func SetClient(Client *http.Client) {
	globalHTTPClient = Client
}

func normalizeLang(from, to string, withVerification bool) (string, string) {
	if !withVerification {
		if from == "" {
			from = "auto"
		}
		return from, to
	}
	if _, err := language.Parse(from); err != nil && from != "auto" {
		from = "auto"
	}
	if _, err := language.Parse(to); err != nil {
		to = "en"
	}
	return from, to
}

func buildLightTranslateURL(text, from, to string) (string, error) {
	u, err := url.Parse(googleTranslateEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client", "gtx")
	q.Set("sl", from)
	q.Set("tl", to)
	q.Set("dt", "t")
	q.Set("dj", "1")
	q.Set("ie", "UTF-8")
	q.Set("q", text)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseLightTranslateResponse(raw []byte) (string, error) {
	var payload struct {
		Sentences []struct {
			Trans string `json:"trans"`
		} `json:"sentences"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}

	responseText := ""
	for _, sentence := range payload.Sentences {
		responseText += sentence.Trans
	}
	if responseText == "" {
		return "", errBadRequest
	}
	return responseText, nil
}

func doLightTranslate(ctx context.Context, client *http.Client, text, from, to string, tries int, delay time.Duration) (string, error) {
	if client == nil {
		client = globalHTTPClient
	}
	if tries == 0 {
		tries = defaultNumberOfRetries
	}

	reqURL, err := buildLightTranslateURL(text, from, to)
	if err != nil {
		return "", err
	}

	for tries > 0 {
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")

		r, err2 := client.Do(req)
		if err2 != nil {
			if errors.Is(err2, http.ErrHandlerTimeout) {
				tries--
				if tries == 0 {
					return "", errBadNetwork
				}
				time.Sleep(delay)
				continue
			}
			if errors.Is(err2, context.Canceled) {
				return "", context.Canceled
			}
			tries--
			if tries == 0 {
				return "", err2
			}
			time.Sleep(delay)
			continue
		}

		if r.StatusCode == http.StatusForbidden {
			tries--
			if tries == 0 {
				r.Body.Close()
				return "", fmt.Errorf("too many forbidden requests")
			}
			r.Body.Close()
			time.Sleep(delay)
			continue
		}

		if r.StatusCode != http.StatusOK {
			tries--
			if tries == 0 {
				raw, _ := ioutil.ReadAll(r.Body)
				r.Body.Close()
				return "", fmt.Errorf("unexpected status code: %d, body: %s", r.StatusCode, string(raw))
			}
			r.Body.Close()
			time.Sleep(delay)
			continue
		}

		raw, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return "", err
		}

		return parseLightTranslateResponse(raw)
	}

	return "", fmt.Errorf("max retries exceeded")
}

func translate(ctx context.Context, text, from, to string, withVerification bool, tries int, delay time.Duration) (string, error) {
	from, to = normalizeLang(from, to, withVerification)
	return doLightTranslate(ctx, globalHTTPClient, text, from, to, tries, delay)
}
