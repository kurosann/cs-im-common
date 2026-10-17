package libretranslate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/translate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var req translateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Q != "hello" || req.Source != "en" || req.Target != "zh" || req.APIKey != "secret" {
			t.Fatalf("unexpected request: %+v", req)
		}

		_ = json.NewEncoder(w).Encode(translateResponse{TranslatedText: "ni hao"})
	}))
	defer server.Close()

	got, err := NewClient(server.URL, WithAPIKey("secret")).Translate(context.Background(), "hello", "en", "zh")
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if got != "ni hao" {
		t.Fatalf("Translate = %q, want %q", got, "ni hao")
	}
}

func TestTranslateWithParamsDefaultsSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req translateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Source != "auto" {
			t.Fatalf("source = %q, want auto", req.Source)
		}
		if req.Format != "text" {
			t.Fatalf("format = %q, want text", req.Format)
		}

		_ = json.NewEncoder(w).Encode(translateResponse{TranslatedText: "bonjour"})
	}))
	defer server.Close()

	got, err := TranslateWithParams(context.Background(), TranslationParams{
		Host:   server.URL,
		Text:   "hello",
		Target: "fr",
	})
	if err != nil {
		t.Fatalf("TranslateWithParams returned error: %v", err)
	}
	if got != "bonjour" {
		t.Fatalf("TranslateWithParams = %q, want %q", got, "bonjour")
	}
}

func TestBatchTranslateKeepsOrderAndLimitsConcurrency(t *testing.T) {
	var inFlight int32
	var maxInFlight int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)

		for {
			maxSeen := atomic.LoadInt32(&maxInFlight)
			if current <= maxSeen || atomic.CompareAndSwapInt32(&maxInFlight, maxSeen, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)

		var req translateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(translateResponse{TranslatedText: "translated-" + req.Q})
	}))
	defer server.Close()

	params := make([]TranslationParams, 20)
	for i := range params {
		params[i] = TranslationParams{
			Text:   strconv.Itoa(i),
			Source: "en",
			Target: "zh",
		}
	}

	results, err := NewClient(server.URL).BatchTranslate(context.Background(), params, 3)
	if err != nil {
		t.Fatalf("BatchTranslate returned error: %v", err)
	}
	if atomic.LoadInt32(&maxInFlight) > 3 {
		t.Fatalf("max in-flight requests = %d, want <= 3", maxInFlight)
	}
	for i, result := range results {
		want := fmt.Sprintf("translated-%d", i)
		if result.TranslatedText != want {
			t.Fatalf("result[%d] = %q, want %q", i, result.TranslatedText, want)
		}
	}
}

func TestClientTranslateConcurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req translateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(translateResponse{TranslatedText: req.Q})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			text := strconv.Itoa(i)
			got, err := client.Translate(context.Background(), text, "en", "zh")
			if err != nil {
				t.Errorf("Translate returned error: %v", err)
				return
			}
			if got != text {
				t.Errorf("Translate = %q, want %q", got, text)
			}
		}(i)
	}
	wg.Wait()
}

func TestClientTranslateUsesQueue(t *testing.T) {
	var inFlight int32
	var maxInFlight int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)

		for {
			maxSeen := atomic.LoadInt32(&maxInFlight)
			if current <= maxSeen || atomic.CompareAndSwapInt32(&maxInFlight, maxSeen, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(translateResponse{TranslatedText: "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, WithConcurrency(1))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tranText, err := client.Translate(context.Background(), "hello", "en", "zh")
			if err != nil {
				t.Errorf("Translate returned error: %v", err)
			}
			t.Logf("Translated %s: %s", "hello", tranText)
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&maxInFlight) > 2 {
		t.Fatalf("max in-flight requests = %d, want <= 2", maxInFlight)
	}
}

func TestLanguages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/languages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Language{{Code: "en", Name: "English"}})
	}))
	defer server.Close()

	languages, err := NewClient(server.URL).Languages(context.Background())
	if err != nil {
		t.Fatalf("Languages returned error: %v", err)
	}
	if len(languages) != 1 || languages[0].Code != "en" {
		t.Fatalf("unexpected languages: %+v", languages)
	}
}

func TestDetect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/detect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Detection{{Language: "en", Confidence: 99.9}})
	}))
	defer server.Close()

	detections, err := NewClient(server.URL).Detect(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if len(detections) != 1 || detections[0].Language != "en" {
		t.Fatalf("unexpected detections: %+v", detections)
	}
}
