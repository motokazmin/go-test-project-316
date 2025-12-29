package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"

	"code/internal/httputil"
	"code/internal/parser"
)

// Asset содержит информацию об ассете (картинка, скрипт, стиль)
type Asset struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error,omitempty"`
}

// AssetResult содержит результат проверки ассета
type AssetResult struct {
	URL        string
	Type       string
	StatusCode int
	SizeBytes  int64
	Error      error
}

// AssetChecker проверяет ассеты и кэширует результаты
type AssetChecker struct {
	fetcher    *httputil.Fetcher
	parser     *parser.HTMLParser
	workers    int
	cache      map[string]Asset
	cacheMutex sync.RWMutex
}

// NewAssetChecker создает новый checker для ассетов
func NewAssetChecker(fetcher *httputil.Fetcher, htmlParser *parser.HTMLParser, workers int) *AssetChecker {
	return &AssetChecker{
		fetcher: fetcher,
		parser:  htmlParser,
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
	assetInfos := ac.parser.ExtractAssets(htmlContent, pageURL)

	if len(assetInfos) == 0 {
		return []Asset{}
	}

	// Проверяем ассеты параллельно с учетом кэша и сохранением порядка
	semaphore := make(chan struct{}, ac.workers)
	resultChan := make(chan assetWithIndex, len(assetInfos))
	var wg sync.WaitGroup

	for i, info := range assetInfos {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(index int, assetURL, assetType string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			asset := ac.checkSingleAsset(ctx, assetURL, assetType)
			resultChan <- assetWithIndex{asset: asset, index: index}
		}(i, info.URL, info.AssetType)
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
	assets := make([]Asset, len(assetInfos))
	for i := 0; i < len(assetInfos); i++ {
		assets[i] = results[i]
	}

	// Сортируем по типу (алфавитный порядок: image, script, style)
	sort.SliceStable(assets, func(i, j int) bool {
		return assets[i].Type < assets[j].Type
	})

	return assets
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
	if rl := ac.fetcher.RateLimiter(); rl != nil {
		if !rl.Wait(ctx) {
			return AssetResult{Error: ctx.Err()}
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, ac.fetcher.Timeout())
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, assetURL, nil)
	if err != nil {
		return AssetResult{Error: err}
	}

	if ac.fetcher.UserAgent() != "" {
		req.Header.Set("User-Agent", ac.fetcher.UserAgent())
	}

	resp, err := ac.fetcher.Client().Do(req)
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
