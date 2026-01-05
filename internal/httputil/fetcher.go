package httputil

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient — интерфейс для выполнения HTTP запросов
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// FetchResult содержит результат HTTP-запроса
type FetchResult struct {
	StatusCode  int
	HTMLContent string
	Error       error
}

type FetcherConfig struct {
	Client     HTTPClient
	UserAgent  string
	Timeout    time.Duration
	MaxRetries int
}

// Fetcher выполняет HTTP-запросы с retry логикой и rate limiting
type Fetcher struct {
	client      HTTPClient
	userAgent   string
	timeout     time.Duration
	maxRetries  int
	rateLimiter *RateLimiter
}

func NewFetcher(cfg FetcherConfig, rateLimiter *RateLimiter) *Fetcher {
	return &Fetcher{
		client:      cfg.Client,
		userAgent:   cfg.UserAgent,
		timeout:     cfg.Timeout,
		maxRetries:  cfg.MaxRetries,
		rateLimiter: rateLimiter,
	}
}

func (f *Fetcher) Timeout() time.Duration {
	return f.timeout
}

func (f *Fetcher) UserAgent() string {
	return f.userAgent
}

func (f *Fetcher) Client() HTTPClient {
	return f.client
}

func (f *Fetcher) RateLimiter() *RateLimiter {
	return f.rateLimiter
}

// Fetch выполняет HTTP-запрос с retry логикой.
// Retry выполняется при: сетевых ошибках, HTTP 429, HTTP 5xx.
func (f *Fetcher) Fetch(ctx context.Context, url string) FetchResult {
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return FetchResult{Error: ctx.Err()}
		}

		// Задержка 100ms перед повторной попыткой
		if attempt > 0 {
			if !f.waitForRetry(ctx) {
				return FetchResult{Error: ctx.Err()}
			}
		}

		result := f.performRequest(ctx, url)

		// Успех — не требует retry
		if result.Error == nil && result.StatusCode < 500 && result.StatusCode != 429 {
			return result
		}

		if attempt == f.maxRetries || !f.shouldRetry(result) {
			return result
		}
	}

	return FetchResult{}
}

// shouldRetry: сетевые ошибки, 429 Too Many Requests, 5xx Server Errors
func (f *Fetcher) shouldRetry(result FetchResult) bool {
	if result.Error != nil {
		return true
	}
	if result.StatusCode == 429 {
		return true
	}
	if result.StatusCode >= 500 && result.StatusCode < 600 {
		return true
	}
	return false
}

func (f *Fetcher) performRequest(ctx context.Context, urlStr string) FetchResult {
	if f.rateLimiter != nil {
		if !f.rateLimiter.Wait(ctx) {
			return FetchResult{Error: ctx.Err()}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return FetchResult{Error: err}
	}

	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return FetchResult{Error: err}
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	result := FetchResult{StatusCode: resp.StatusCode}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if isTextContent(resp.Header.Get("Content-Type")) {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				result.HTMLContent = string(body)
			}
		}
	}

	return result
}

func (f *Fetcher) waitForRetry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(100 * time.Millisecond):
		return true
	}
}

func isTextContent(contentType string) bool {
	return strings.Contains(contentType, "text/html") || strings.Contains(contentType, "xml")
}
