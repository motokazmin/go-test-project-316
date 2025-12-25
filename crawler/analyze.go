package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Analyze анализирует структуру веб-сайта
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	// 1. Нормализуем опции
	normalizeOptions(&opts)

	fmt.Printf("[DEBUG] Analyze called with URL=%s, Depth=%d\n", opts.URL, opts.Depth)

	// 2. Валидируем URL
	rootURL, err := ParseAndValidateURL(opts.URL)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[DEBUG] Root URL normalized: %s\n", rootURL.String())

	// 3. Создаем компоненты
	rateLimiter := NewRateLimiter(ctx, opts.Delay)
	state := NewCrawlState(rootURL, opts.Concurrency, rateLimiter)
	fmt.Printf("[DEBUG] Base URL for domain check: %s\n", state.BaseURL.String())
	fetcher := NewFetcher(opts, rateLimiter)
	parser := NewHTMLParser()
	seoExtractor := NewSEOExtractor()
	linkChecker := NewLinkChecker(fetcher, opts.Concurrency)
	reportBuilder := NewReportBuilder(rootURL, opts.Depth)
	assetChecker := NewAssetChecker(fetcher, opts.Concurrency)

	// 4. Создаем crawler
	crawler := &Crawler{
		state:         state,
		fetcher:       fetcher,
		parser:        parser,
		seoExtractor:  seoExtractor,
		linkChecker:   linkChecker,
		assetChecker:  assetChecker,
		reportBuilder: reportBuilder,
		maxDepth:      opts.Depth,
	}

	fmt.Printf("[DEBUG] Crawler created with maxDepth=%d\n", crawler.maxDepth)

	// 5. Запускаем обход
	crawler.Run(ctx)

	fmt.Printf("[DEBUG] Crawl finished\n")

	// 6. Возвращаем отчет
	return reportBuilder.Encode(opts.IndentJSON)
}

// Crawler координирует процесс обхода сайта
type Crawler struct {
	state         *CrawlState
	fetcher       *Fetcher
	parser        *HTMLParser
	seoExtractor  *SEOExtractor
	linkChecker   *LinkChecker
	assetChecker  *AssetChecker
	reportBuilder *ReportBuilder
	maxDepth      int
}

// Run запускает процесс обхода
func (c *Crawler) Run(ctx context.Context) {
	iteration := 0
	for {
		iteration++
		fmt.Printf("[DEBUG] Run iteration %d, queue empty: %v\n", iteration, c.state.Queue.IsEmpty())

		// Проверяем контекст
		if ctx.Err() != nil {
			fmt.Printf("[DEBUG] Context cancelled\n")
			break
		}

		// Берём URL из очереди
		item := c.state.Queue.Dequeue()

		// Если очереди пуста, ждём завершения всех воркеров
		if item == nil {
			fmt.Printf("[DEBUG] Queue empty, waiting for workers...\n")
			c.state.WG.Wait()
			fmt.Printf("[DEBUG] All workers done\n")

			// После завершения воркеров проверяем очередь снова
			// (воркеры могли добавить новые URL)
			if c.state.Queue.IsEmpty() {
				fmt.Printf("[DEBUG] Queue still empty after workers, exiting\n")
				break
			}
			fmt.Printf("[DEBUG] Queue has items after workers, continuing\n")
			continue
		}

		fmt.Printf("[DEBUG] Processing URL: %s (depth=%d)\n", item.url, item.depth)

		// Обрабатываем URL
		c.processURLWithWorker(ctx, item.url, item.depth)
	}

	// Финальное ожидание завершения всех воркеров
	c.state.WG.Wait()
	fmt.Printf("[DEBUG] Final wait done\n")
}

// processURLWithWorker обрабатывает URL в отдельном worker'е
func (c *Crawler) processURLWithWorker(ctx context.Context, urlStr string, depth int) {
	c.state.WG.Add(1)
	c.state.Semaphore <- struct{}{}

	go func() {
		defer c.state.WG.Done()
		defer func() { <-c.state.Semaphore }()

		c.processSingleURL(ctx, urlStr, depth)
	}()
}

// processSingleURL обрабатывает один URL
func (c *Crawler) processSingleURL(ctx context.Context, urlStr string, depth int) {
	select {
	case <-ctx.Done():
		fmt.Printf("[DEBUG] Worker cancelled for %s\n", urlStr)
		return
	default:
	}

	if c.state.Visited.Contains(urlStr) {
		fmt.Printf("[DEBUG] Already visited: %s\n", urlStr)
		return
	}

	fmt.Printf("[DEBUG] Visiting: %s (depth=%d)\n", urlStr, depth)
	c.state.Visited.Add(urlStr)

	page := Page{
		URL:   urlStr,
		Depth: depth,
	}

	// Выполняем HTTP-запрос
	result := c.fetcher.Fetch(ctx, urlStr)
	page.HTTPStatus = result.StatusCode

	fmt.Printf("[DEBUG] Fetched %s: status=%d, hasContent=%v\n", urlStr, result.StatusCode, result.HTMLContent != "")

	// Логируем HTML контент для отладки
	if result.HTMLContent != "" {
		fmt.Printf("[DEBUG] HTML Content (first 500 chars):\n%s\n", truncateString(result.HTMLContent, 500))
		fmt.Printf("[DEBUG] HTML Content (full length): %d bytes\n", len(result.HTMLContent))
	}

	if result.Error != nil {
		page.Error = result.Error.Error()
		SetPageStatus(&page)
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		page.SEO = &SEO{}
		page.BrokenLinks = []BrokenLink{}
		page.Assets = []Asset{}
		c.reportBuilder.AddPage(page)
		fmt.Printf("[DEBUG] Added page with error: %s\n", urlStr)
		return
	}

	SetPageStatus(&page)

	if result.HTMLContent != "" {
		pageURL, _ := url.Parse(urlStr)

		// Извлекаем SEO данные
		page.SEO = c.seoExtractor.Extract(result.HTMLContent)

		// Извлекаем ссылки
		links := c.parser.ExtractLinks(result.HTMLContent, pageURL)
		fmt.Printf("[DEBUG] Found %d links on %s\n", len(links), urlStr)

		// Логируем каждую ссылку
		for i, link := range links {
			fmt.Printf("[DEBUG]   Link %d: %s\n", i+1, link)
		}

		// Проверяем битые ссылки (все ссылки)
		page.BrokenLinks, page.DiscoveredAt = c.linkChecker.CheckLinks(ctx, links)
		fmt.Printf("[DEBUG] Found %d broken links\n", len(page.BrokenLinks))
		for i, bl := range page.BrokenLinks {
			fmt.Printf("[DEBUG]   Broken link %d: %s (status=%d, error=%s)\n", i+1, bl.URL, bl.StatusCode, bl.Error)
		}

		// Проверяем ассеты
		page.Assets = c.assetChecker.CheckAssets(ctx, result.HTMLContent, pageURL)

		// Добавляем внутренние ссылки в очередь
		// depth+1 < maxDepth означает: следующий уровень не превысит maxDepth
		shouldEnqueue := depth+1 < c.maxDepth && page.Status == "ok"
		fmt.Printf("[DEBUG] Should enqueue links? depth=%d, maxDepth=%d, depth+1=%d, status=%s → %v\n",
			depth, c.maxDepth, depth+1, page.Status, shouldEnqueue)

		if shouldEnqueue {
			c.enqueueInternalLinks(links, depth+1)
		}
	} else {
		// Если нет HTML контента, но запрос был успешен
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		page.SEO = &SEO{}
		page.BrokenLinks = []BrokenLink{}
		page.Assets = []Asset{}
	}

	c.reportBuilder.AddPage(page)
	fmt.Printf("[DEBUG] Added page: %s\n", urlStr)
}

// enqueueInternalLinks добавляет внутренние ссылки в очередь
func (c *Crawler) enqueueInternalLinks(links []string, depth int) {
	toAdd := []urlWithDepth{}

	fmt.Printf("[DEBUG] enqueueInternalLinks called with %d links, depth=%d\n", len(links), depth)

	for _, link := range links {
		linkURL, err := url.Parse(link)
		if err != nil || !IsSameDomain(linkURL, c.state.BaseURL) {
			continue
		}

		// Нормализуем URL перед добавлением
		normalized := NormalizeURL(linkURL)
		if !c.state.Visited.Contains(normalized) {
			fmt.Printf("[DEBUG] Will enqueue: %s (depth=%d)\n", normalized, depth)
			toAdd = append(toAdd, urlWithDepth{url: normalized, depth: depth})
		} else {
			fmt.Printf("[DEBUG] Already visited, skip: %s\n", normalized)
		}
	}

	if len(toAdd) > 0 {
		fmt.Printf("[DEBUG] Enqueueing %d URLs\n", len(toAdd))
		c.state.Queue.Enqueue(toAdd)
	} else {
		fmt.Printf("[DEBUG] No URLs to enqueue\n")
	}
}

// normalizeOptions устанавливает значения по умолчанию
func normalizeOptions(opts *Options) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
}

// truncateString обрезает строку до maxLen символов
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
