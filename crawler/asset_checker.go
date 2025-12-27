package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
)

// AssetChecker проверяет ассеты и кэширует результаты
type AssetChecker struct {
	fetcher    *Fetcher
	workers    int
	cache      map[string]Asset
	cacheMutex sync.RWMutex
}

// NewAssetChecker создает новый checker для ассетов
func NewAssetChecker(fetcher *Fetcher, workers int) *AssetChecker {
	return &AssetChecker{
		fetcher: fetcher,
		workers: workers,
		cache:   make(map[string]Asset),
	}
}

// assetWithIndex содержит ассет и его исходный индекс
type assetWithIndex struct {
	asset Asset
	index int
}

// CheckAssets проверяет все ассеты на странице
func (ac *AssetChecker) CheckAssets(ctx context.Context, htmlContent string, pageURL *url.URL) []Asset {
	// Извлекаем ассеты из HTML
	assetURLs := ac.extractAssetURLs(htmlContent, pageURL)

	if len(assetURLs) == 0 {
		return []Asset{}
	}

	// Проверяем ассеты параллельно с учетом кэша и сохранением порядка
	semaphore := make(chan struct{}, ac.workers)
	resultChan := make(chan assetWithIndex, len(assetURLs))
	var wg sync.WaitGroup

	for i, assetInfo := range assetURLs {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(index int, url, assetType string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			asset := ac.checkSingleAsset(ctx, url, assetType)
			resultChan <- assetWithIndex{asset: asset, index: index}
		}(i, assetInfo.url, assetInfo.assetType)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Собираем результаты в map для сохранения порядка
	results := make(map[int]Asset)
	for result := range resultChan {
		results[result.index] = result.asset
	}

	// Восстанавливаем исходный порядок
	assets := make([]Asset, len(assetURLs))
	for i := 0; i < len(assetURLs); i++ {
		assets[i] = results[i]
	}

	// Сортируем по типу (алфавитный порядок: image, script, style)
	sort.SliceStable(assets, func(i, j int) bool {
		return assets[i].Type < assets[j].Type
	})

	return assets
}

// assetInfo содержит информацию об ассете из HTML
type assetInfo struct {
	url       string
	assetType string
}

// extractAssetURLs извлекает URL ассетов из HTML
func (ac *AssetChecker) extractAssetURLs(htmlContent string, pageURL *url.URL) []assetInfo {
	parser := NewHTMLParser()
	return parser.ExtractAssets(htmlContent, pageURL)
}

// checkSingleAsset проверяет один ассет с использованием кэша
func (ac *AssetChecker) checkSingleAsset(ctx context.Context, assetURL, assetType string) Asset {
	// Проверяем кэш (читающая блокировка)
	ac.cacheMutex.RLock()
	cached, found := ac.cache[assetURL]
	ac.cacheMutex.RUnlock()

	if found {
		return cached
	}

	// Выполняем запрос
	result := ac.fetchAsset(ctx, assetURL)

	// Формируем Asset
	asset := Asset{
		URL:        assetURL,
		Type:       assetType,
		StatusCode: result.StatusCode,
		SizeBytes:  result.SizeBytes,
	}

	if result.Error != nil {
		asset.Error = result.Error.Error()
	}

	// Сохраняем в кэш (пишущая блокировка)
	ac.cacheMutex.Lock()
	ac.cache[assetURL] = asset
	ac.cacheMutex.Unlock()

	return asset
}

// fetchAsset выполняет HTTP-запрос для ассета
func (ac *AssetChecker) fetchAsset(ctx context.Context, assetURL string) AssetResult {
	// Rate limiting через fetcher
	if ac.fetcher.rateLimiter != nil {
		if !ac.fetcher.rateLimiter.Wait(ctx) {
			return AssetResult{Error: ctx.Err()}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, ac.fetcher.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, assetURL, nil)
	if err != nil {
		return AssetResult{Error: err}
	}

	if ac.fetcher.userAgent != "" {
		req.Header.Set("User-Agent", ac.fetcher.userAgent)
	}

	resp, err := ac.fetcher.client.Do(req)
	if err != nil {
		return AssetResult{Error: err}
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	result := AssetResult{
		StatusCode: resp.StatusCode,
	}

	// Для ошибочных статусов не читаем тело
	if resp.StatusCode >= 400 {
		result.Error = fmt.Errorf("HTTP %d", resp.StatusCode)
		return result
	}

	// Пытаемся получить размер из Content-Length
	contentLength := resp.ContentLength

	if contentLength >= 0 {
		// Content-Length присутствует
		result.SizeBytes = contentLength

		// Читаем и отбрасываем тело для освобождения соединения
		_, _ = io.Copy(io.Discard, resp.Body)
	} else {
		// Content-Length отсутствует - читаем тело и считаем размер
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Errorf("failed to read body: %w", err)
			return result
		}
		result.SizeBytes = int64(len(body))
	}

	return result
}
