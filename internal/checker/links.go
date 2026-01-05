package checker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"code/internal/httputil"
)

// BrokenLink содержит информацию о битой ссылке
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

// LinkChecker проверяет доступность ссылок
type LinkChecker struct {
	fetcher *httputil.Fetcher
	workers int
}

func NewLinkChecker(fetcher *httputil.Fetcher, workers int) *LinkChecker {
	return &LinkChecker{
		fetcher: fetcher,
		workers: workers,
	}
}

// CheckLinks проверяет список ссылок параллельно.
// Возвращает только битые ссылки (после всех retry).
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

func (lc *LinkChecker) checkSingleLink(ctx context.Context, linkURL string) (BrokenLink, bool) {
	result := lc.headRequest(ctx, linkURL)

	if result.StatusCode >= 200 && result.StatusCode < 400 && result.Error == nil {
		return BrokenLink{}, false
	}

	brokenLink := BrokenLink{URL: linkURL}
	if result.Error != nil {
		brokenLink.Error = result.Error.Error()
	} else {
		brokenLink.StatusCode = result.StatusCode
	}

	return brokenLink, true
}

func (lc *LinkChecker) headRequest(ctx context.Context, urlStr string) httputil.FetchResult {
	maxRetries := 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return httputil.FetchResult{Error: ctx.Err()}
		}

		if attempt > 0 {
			select {
			case <-ctx.Done():
				return httputil.FetchResult{Error: ctx.Err()}
			case <-time.After(100 * time.Millisecond):
			}
		}

		result := lc.performHeadRequest(ctx, urlStr)

		if result.Error == nil && result.StatusCode < 500 && result.StatusCode != 429 {
			return result
		}

		if attempt == maxRetries || !lc.shouldRetry(result) {
			return result
		}
	}

	return httputil.FetchResult{}
}

func (lc *LinkChecker) shouldRetry(result httputil.FetchResult) bool {
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

func (lc *LinkChecker) performHeadRequest(ctx context.Context, urlStr string) httputil.FetchResult {
	if rl := lc.fetcher.RateLimiter(); rl != nil {
		if !rl.Wait(ctx) {
			return httputil.FetchResult{Error: ctx.Err()}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, lc.fetcher.Timeout())
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodHead, urlStr, nil)
	if err != nil {
		return httputil.FetchResult{Error: err}
	}

	if lc.fetcher.UserAgent() != "" {
		req.Header.Set("User-Agent", lc.fetcher.UserAgent())
	}

	resp, err := lc.fetcher.Client().Do(req)
	if err != nil {
		return httputil.FetchResult{Error: err}
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	return httputil.FetchResult{
		StatusCode: resp.StatusCode,
	}
}
