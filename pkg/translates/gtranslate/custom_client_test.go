package gtranslate

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTranslateWithParamsUsesCustomClientForLightAPI(t *testing.T) {
	oldGlobalClient := globalHTTPClient
	oldGoogleHost := GoogleHost

	t.Cleanup(func() {
		globalHTTPClient = oldGlobalClient
		GoogleHost = oldGoogleHost
	})

	globalHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("globalHTTPClient should not be used, got request: %s", req.URL.String())
			return nil, nil
		}),
	}

	var (
		mu    sync.Mutex
		calls []string
	)

	customClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			calls = append(calls, req.URL.String())
			mu.Unlock()

			if req.URL.Host != "translate.googleapis.com" || req.URL.Path != "/translate_a/single" {
				t.Fatalf("unexpected request: %s", req.URL.String())
			}
			q := req.URL.Query()
			if q.Get("client") != "gtx" || q.Get("dt") != "t" || q.Get("dj") != "1" {
				t.Fatalf("unexpected query: %s", req.URL.RawQuery)
			}
			if q.Get("tk") != "" {
				t.Fatalf("light API should not send tk, got %q", q.Get("tk"))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sentences":[{"trans":"你好"}],"src":"en"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	got, err := TranslateWithParams(context.Background(), "hello", TranslationParams{
		Client:     customClient,
		From:       "en",
		To:         "zh",
		Tries:      1,
		Delay:      0,
		GoogleHost: "google.com",
	})
	if err != nil {
		t.Fatalf("TranslateWithParams returned error: %v", err)
	}

	if got != "你好" {
		t.Fatalf("expected translated text to be 你好, got %q", got)
	}

	if len(calls) != 1 {
		t.Fatalf("expected custom client to receive 1 request, got %d (%v)", len(calls), calls)
	}
}
