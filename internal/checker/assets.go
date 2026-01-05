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

// Asset содержит информацию об ассете (изображение, скрипт, стиль)
type Asset struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error,omitempty"`
}

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

func NewAssetChecker(fetcher *httputil.Fetcher, htmlParser *parser.HTMLParser, workers int) *AssetChecker {
	return &AssetChecker{
		fetcher: fetcher,
		parser:  htmlParser,
		workers: workers,
		cache:   make(map[string]Asset),
	}
}

type assetWithIndex struct {
	asset Asset
	index int
}

// CheckAssets извлекает и проверяет все ассеты на странице
func (ac *AssetChecker) CheckAssets(ctx context.Context, htmlContent string, pageURL *url.URL) []Asset {
	assetInfos := ac.parser.ExtractAssets(htmlContent, pageURL)

	if len(assetInfos) == 0 {
		return []Asset{}
	}

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

	results := make(map[int]Asset)
	for result := range resultChan {
		results[result.index] = result.asset
	}

	assets := make([]Asset, len(assetInfos))
	for i := 0; i < len(assetInfos); i++ {
		assets[i] = results[i]
	}

	sort.SliceStable(assets, func(i, j int) bool {
		return assets[i].Type < assets[j].Type
	})

	return assets
}

func (ac *AssetChecker) checkSingleAsset(ctx context.Context, assetURL, assetType string) Asset {
	ac.cacheMutex.RLock()
	cached, found := ac.cache[assetURL]
	ac.cacheMutex.RUnlock()

	if found {
		return cached
	}

	result := ac.fetchAsset(ctx, assetURL)

	asset := Asset{
		URL:        assetURL,
		Type:       assetType,
		StatusCode: result.StatusCode,
		SizeBytes:  result.SizeBytes,
	}

	if result.Error != nil {
		asset.Error = result.Error.Error()
	}

	ac.cacheMutex.Lock()
	ac.cache[assetURL] = asset
	ac.cacheMutex.Unlock()

	return asset
}

func (ac *AssetChecker) fetchAsset(ctx context.Context, assetURL string) AssetResult {
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

	if resp.StatusCode >= 400 {
		result.Error = fmt.Errorf("HTTP %d", resp.StatusCode)
		return result
	}

	contentLength := resp.ContentLength

	if contentLength >= 0 {
		result.SizeBytes = contentLength
		_, _ = io.Copy(io.Discard, resp.Body)
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			result.Error = fmt.Errorf("failed to read body: %w", err)
			return result
		}
		result.SizeBytes = int64(len(body))
	}

	return result
}
