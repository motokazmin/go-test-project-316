package crawler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Fetcher отвечает за выполнение HTTP-запросов
type Fetcher struct {
	client      HTTPClient
	userAgent   string
	timeout     time.Duration
	maxRetries  int
	rateLimiter *RateLimiter
}

// NewFetcher создает новый HTTP fetcher
func NewFetcher(opts Options, rateLimiter *RateLimiter) *Fetcher {
	return &Fetcher{
		client:      opts.HTTPClient,
		userAgent:   opts.UserAgent,
		timeout:     opts.Timeout,
		maxRetries:  opts.Retries,
		rateLimiter: rateLimiter,
	}
}

// Fetch выполняет HTTP-запрос с retry логикой и rate limiting
func (f *Fetcher) Fetch(ctx context.Context, url string) FetchResult {
	var lastErr error

	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		if attempt > 0 {
			if !f.waitForRetry(ctx) {
				return FetchResult{Error: ctx.Err()}
			}
		}

		result := f.performRequest(ctx, url)
		if result.Error == nil {
			return result
		}

		lastErr = result.Error
	}

	return FetchResult{Error: lastErr}
}

// performRequest выполняет один HTTP-запрос
func (f *Fetcher) performRequest(ctx context.Context, urlStr string) FetchResult {
	// ✅ КРИТИЧНО: Rate limiting перед HTTP-запросом
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
	defer resp.Body.Close()

	result := FetchResult{StatusCode: resp.StatusCode}

	// Читаем HTML контент только для успешных ответов
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if isHTMLContent(resp.Header.Get("Content-Type")) {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				result.HTMLContent = string(body)
			}
		}
	}

	return result
}

// waitForRetry ждет перед повтором
func (f *Fetcher) waitForRetry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(100 * time.Millisecond):
		return true
	}
}

// isHTMLContent проверяет тип контента
func isHTMLContent(contentType string) bool {
	return strings.Contains(contentType, "text/html")
}
