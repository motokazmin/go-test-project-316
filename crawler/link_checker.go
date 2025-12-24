package crawler

import (
	"context"
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
	// Fetch автоматически выполняет retry
	result := lc.fetcher.Fetch(ctx, linkURL)

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
