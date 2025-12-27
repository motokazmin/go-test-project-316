package crawler

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// LinkChecker проверяет доступность ссылок
type LinkChecker struct {
	fetcher *Fetcher
	workers int
}

// NewLinkChecker создает новый checker
func NewLinkChecker(fetcher *Fetcher, workers int) *LinkChecker {
	return &LinkChecker{
		fetcher: fetcher,
		workers: workers,
	}
}

// CheckLinks проверяет список ссылок параллельно
// Возвращает только битые ссылки (после всех retry)
func (lc *LinkChecker) CheckLinks(ctx context.Context, links []string) ([]BrokenLink, string) {
	if len(links) == 0 {
		return nil, time.Now().UTC().Format(time.RFC3339)
	}

	semaphore := make(chan struct{}, lc.workers)
	resultChan := make(chan BrokenLink, len(links))
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(linkURL string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Проверяем ссылку с retry
			if brokenLink, isBroken := lc.checkSingleLink(ctx, linkURL); isBroken {
				resultChan <- brokenLink
			}
		}(link)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	brokenLinks := []BrokenLink{}
	for brokenLink := range resultChan {
		brokenLinks = append(brokenLinks, brokenLink)
	}

	return brokenLinks, time.Now().UTC().Format(time.RFC3339)
}

// checkSingleLink проверяет одну ссылку
// Возвращает результат ПОСЛЕДНЕЙ попытки
func (lc *LinkChecker) checkSingleLink(ctx context.Context, linkURL string) (BrokenLink, bool) {
	result := lc.headRequest(ctx, linkURL)

	// Ссылка работает
	if result.StatusCode >= 200 && result.StatusCode < 400 && result.Error == nil {
		return BrokenLink{}, false
	}

	// Ссылка битая - возвращаем результат последней попытки
	brokenLink := BrokenLink{URL: linkURL}
	if result.Error != nil {
		brokenLink.Error = result.Error.Error()
	} else {
		brokenLink.StatusCode = result.StatusCode
	}

	return brokenLink, true
}

// headRequest выполняет HEAD запрос с retry логикой
func (lc *LinkChecker) headRequest(ctx context.Context, urlStr string) FetchResult {
	maxRetries := 2 // Используем меньше retry для broken links проверки

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Проверяем отмену контекста
		if ctx.Err() != nil {
			return FetchResult{Error: ctx.Err()}
		}

		// Задержка перед повторной попыткой (не для первой)
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return FetchResult{Error: ctx.Err()}
			case <-time.After(100 * time.Millisecond):
			}
		}

		// Выполняем HEAD запрос
		result := lc.performHeadRequest(ctx, urlStr)

		// Успех - возвращаем результат
		if result.Error == nil && result.StatusCode < 500 && result.StatusCode != 429 {
			return result
		}

		// Последняя попытка или не требует retry - возвращаем как есть
		if attempt == maxRetries || !lc.shouldRetry(result) {
			return result
		}
	}

	return FetchResult{}
}

// shouldRetry определяет нужен ли retry
func (lc *LinkChecker) shouldRetry(result FetchResult) bool {
	// Сетевая ошибка - retry
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

	return false
}

// performHeadRequest выполняет один HEAD запрос
func (lc *LinkChecker) performHeadRequest(ctx context.Context, urlStr string) FetchResult {
	// Rate limiting
	if lc.fetcher.rateLimiter != nil {
		if !lc.fetcher.rateLimiter.Wait(ctx) {
			return FetchResult{Error: ctx.Err()}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, lc.fetcher.timeout)
	defer cancel()

	// Используем HEAD вместо GET
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodHead, urlStr, nil)
	if err != nil {
		return FetchResult{Error: err}
	}

	if lc.fetcher.userAgent != "" {
		req.Header.Set("User-Agent", lc.fetcher.userAgent)
	}

	resp, err := lc.fetcher.client.Do(req)
	if err != nil {
		return FetchResult{Error: err}
	}
	defer resp.Body.Close()

	return FetchResult{
		StatusCode: resp.StatusCode,
	}
}
