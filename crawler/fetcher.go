package crawler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Fetcher отвечает за выполнение HTTP-запросов с retry логикой
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
// Между попытками задержка 100ms
func (f *Fetcher) Fetch(ctx context.Context, url string) FetchResult {
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		// Проверяем отмену контекста
		if ctx.Err() != nil {
			return FetchResult{Error: ctx.Err()}
		}

		// Задержка перед повторной попыткой (не для первой)
		if attempt > 0 {
			if !f.waitForRetry(ctx) {
				return FetchResult{Error: ctx.Err()}
			}
		}

		// Выполняем запрос
		result := f.performRequest(ctx, url)

		// Успех - возвращаем результат
		if result.Error == nil && result.StatusCode < 500 && result.StatusCode != 429 {
			return result
		}

		// Последняя попытка или не требует retry - возвращаем как есть
		if attempt == f.maxRetries || !f.shouldRetry(result) {
			return result
		}
	}

	// Недостижимый код (цикл всегда возвращает внутри)
	return FetchResult{}
}

// shouldRetry определяет нужен ли retry для данного результата
func (f *Fetcher) shouldRetry(result FetchResult) bool {
	// Сетевая ошибка - всегда retry
	if result.Error != nil {
		return true
	}

	// HTTP 429 Too Many Requests - retry
	if result.StatusCode == 429 {
		return true
	}

	// HTTP 5xx Server Error - retry
	if result.StatusCode >= 500 && result.StatusCode < 600 {
		return true
	}

	// Остальные статусы - не retry
	// (4xx кроме 429 - это клиентские ошибки, нет смысла повторять)
	return false
}

// performRequest выполняет один HTTP-запрос
func (f *Fetcher) performRequest(ctx context.Context, urlStr string) FetchResult {
	// Rate limiting перед HTTP-запросом
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

// waitForRetry ждет 100ms перед повторной попыткой
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
