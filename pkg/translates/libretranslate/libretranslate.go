package libretranslate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultHost        = "http://translate.tglink.vip:6001"
	defaultTimeout     = 30 * time.Second
	defaultConcurrency = 5
)

// Client sends requests to a LibreTranslate server.
type Client struct {
	host       string
	apiKey     string
	httpClient *http.Client
	queue      chan struct{}
}

var (
	defaultHTTPClient = &http.Client{
		Timeout:   defaultTimeout,
		Transport: defaultTransport(),
	}
	defaultQueue = make(chan struct{}, defaultConcurrency)
)

// Option configures a Client.
type Option func(*Client)

// WithAPIKey configures the LibreTranslate API key.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithHTTPClient configures the HTTP client used by LibreTranslate requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithConcurrency configures the max in-flight translation requests for a client.
func WithConcurrency(concurrency int) Option {
	return func(c *Client) {
		if concurrency > 0 {
			c.queue = make(chan struct{}, concurrency)
		}
	}
}

func withQueue(queue chan struct{}) Option {
	return func(c *Client) {
		if queue != nil {
			c.queue = queue
		}
	}
}

// NewClient creates a LibreTranslate client. Host should be the server base URL,
// for example "http://127.0.0.1:5000".
func NewClient(host string, opts ...Option) *Client {
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultHost
	}

	client := &Client{
		host:       strings.TrimRight(host, "/"),
		httpClient: defaultHTTPClient,
		queue:      make(chan struct{}, defaultConcurrency),
	}
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// BatchTranslate translates multiple items concurrently. Concurrency controls
// how many requests may be in flight at the same time. Results keep the same
// order as params.
func (c *Client) BatchTranslate(ctx context.Context, params []TranslationParams, concurrency int) ([]TranslationResult, error) {
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > len(params) && len(params) > 0 {
		concurrency = len(params)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type job struct {
		index int
		param TranslationParams
	}

	jobs := make(chan job)
	results := make([]TranslationResult, len(params))
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				result, err := c.TranslateWithParams(ctx, item.param)
				if err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("translate index %d: %w", item.index, err)
						cancel()
					})
					continue
				}
				results[item.index] = result
			}
		}()
	}

sendLoop:
	for i, param := range params {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- job{index: i, param: param}:
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// TranslationParams contains parameters accepted by LibreTranslate /translate.
type TranslationParams struct {
	Host         string
	APIKey       string
	Client       *http.Client
	Text         string
	Source       string
	Target       string
	Format       string
	Alternates   int
	Alternatives int
}

type translateRequest struct {
	Q            string `json:"q"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Format       string `json:"format,omitempty"`
	APIKey       string `json:"api_key,omitempty"`
	Alternatives int    `json:"alternatives,omitempty"`
}

type translateResponse struct {
	TranslatedText string   `json:"translatedText"`
	Alternatives   []string `json:"alternatives,omitempty"`
}

// TranslationResult is the parsed response from LibreTranslate /translate.
type TranslationResult struct {
	TranslatedText string
	Alternatives   []string
}

// Translate sends text to LibreTranslate and returns only the translated text.
func (c *Client) Translate(ctx context.Context, text, source, target string) (string, error) {
	result, err := c.TranslateWithParams(ctx, TranslationParams{
		Text:   text,
		Source: source,
		Target: target,
	})
	if err != nil {
		return "", err
	}
	return result.TranslatedText, nil
}

// TranslateWithParams sends text to LibreTranslate and returns the full result.
func (c *Client) TranslateWithParams(ctx context.Context, params TranslationParams) (TranslationResult, error) {
	release, err := acquireQueue(ctx, c.queue)
	if err != nil {
		return TranslationResult{}, err
	}
	defer release()

	if params.Text == "" {
		return TranslationResult{}, fmt.Errorf("text is required")
	}
	if params.Target == "" {
		return TranslationResult{}, fmt.Errorf("target language is required")
	}
	if params.Source == "" {
		params.Source = "auto"
	}
	if params.Format == "" {
		params.Format = "text"
	}
	if params.Alternatives == 0 {
		params.Alternatives = params.Alternates
	}

	reqBody := translateRequest{
		Q:            params.Text,
		Source:       params.Source,
		Target:       params.Target,
		Format:       params.Format,
		APIKey:       c.apiKey,
		Alternatives: params.Alternatives,
	}

	var respBody translateResponse
	if err := c.postJSON(ctx, "/translate", reqBody, &respBody); err != nil {
		return TranslationResult{}, err
	}

	return TranslationResult{
		TranslatedText: respBody.TranslatedText,
		Alternatives:   respBody.Alternatives,
	}, nil
}

// Translate is a helper for one-off translations.
func Translate(ctx context.Context, text, source, target string, apiKey ...string) (string, error) {
	opts := []Option{
		WithHTTPClient(defaultHTTPClient),
		withQueue(defaultQueue),
	}
	if len(apiKey) > 0 && apiKey[0] != "" {
		opts = append(opts, WithAPIKey(apiKey[0]))
	}
	return NewClient(defaultHost, opts...).Translate(ctx, text, source, target)
}

// TranslateWithParams is a helper for one-off translations with all parameters.
func TranslateWithParams(ctx context.Context, params TranslationParams) (string, error) {
	opts := []Option{withQueue(defaultQueue)}
	if params.APIKey != "" {
		opts = append(opts, WithAPIKey(params.APIKey))
	}
	if params.Client != nil {
		opts = append(opts, WithHTTPClient(params.Client))
	}

	result, err := NewClient(params.Host, opts...).TranslateWithParams(ctx, params)
	if err != nil {
		return "", err
	}
	return result.TranslatedText, nil
}

// Language describes a language returned by LibreTranslate /languages.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Languages returns languages supported by the LibreTranslate server.
func (c *Client) Languages(ctx context.Context) ([]Language, error) {
	var languages []Language
	if err := c.getJSON(ctx, "/languages", &languages); err != nil {
		return nil, err
	}
	return languages, nil
}

// Detection describes a single language detection candidate.
type Detection struct {
	Confidence float64 `json:"confidence"`
	Language   string  `json:"language"`
}

// Detect sends text to LibreTranslate /detect.
func (c *Client) Detect(ctx context.Context, text string) ([]Detection, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	reqBody := map[string]string{"q": text}
	if c.apiKey != "" {
		reqBody["api_key"] = c.apiKey
	}

	var detections []Detection
	if err := c.postJSON(ctx, "/detect", reqBody, &detections); err != nil {
		return nil, err
	}
	return detections, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) postJSON(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("libretranslate request failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode libretranslate response: %w", err)
	}

	return nil
}

func (c *Client) endpoint(path string) string {
	base, err := url.Parse(c.host)
	if err != nil {
		return c.host + path
	}
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	rel, err := url.Parse(strings.TrimLeft(path, "/"))
	if err != nil {
		return c.host + path
	}
	return base.ResolveReference(rel).String()
}

func acquireQueue(ctx context.Context, queue chan struct{}) (func(), error) {
	if queue == nil {
		return func() {}, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case queue <- struct{}{}:
		return func() { <-queue }, nil
	}
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
}
