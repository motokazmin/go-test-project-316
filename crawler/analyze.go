package crawler

import (
	"context"
	"net/url"
	"time"

	"code/internal/checker"
	"code/internal/httputil"
	"code/internal/parser"
	"code/internal/report"
	"code/internal/seo"
	"code/internal/state"
	"code/internal/urlutil"
)

// Analyze анализирует структуру веб-сайта
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	// 1. Нормализуем опции
	normalizeOptions(&opts)

	// 2. Валидируем URL
	rootURL, err := urlutil.ParseAndValidateURL(opts.URL)
	if err != nil {
		return nil, err
	}

	// 3. Создаем компоненты
	rateLimiter := httputil.NewRateLimiter(ctx, opts.Delay)

	fetcherCfg := httputil.FetcherConfig{
		Client:     opts.HTTPClient,
		UserAgent:  opts.UserAgent,
		Timeout:    opts.Timeout,
		MaxRetries: opts.Retries,
	}
	fetcher := httputil.NewFetcher(fetcherCfg, rateLimiter)

	crawlState := state.NewCrawlState(rootURL, opts.Concurrency, rateLimiter)
	htmlParser := parser.NewHTMLParser()
	seoExtractor := seo.NewExtractor()
	linkChecker := checker.NewLinkChecker(fetcher, opts.Concurrency)
	assetChecker := checker.NewAssetChecker(fetcher, htmlParser, opts.Concurrency)
	reportBuilder := report.NewBuilder(rootURL, opts.Depth)

	// 4. Создаем crawler
	crawler := &Crawler{
		state:         crawlState,
		fetcher:       fetcher,
		parser:        htmlParser,
		seoExtractor:  seoExtractor,
		linkChecker:   linkChecker,
		assetChecker:  assetChecker,
		reportBuilder: reportBuilder,
		maxDepth:      opts.Depth,
	}

	// 5. Запускаем обход
	crawler.Run(ctx)

	// 6. Возвращаем отчет
	return reportBuilder.Encode(opts.IndentJSON)
}

// Crawler координирует процесс обхода сайта
type Crawler struct {
	state         *state.CrawlState
	fetcher       *httputil.Fetcher
	parser        *parser.HTMLParser
	seoExtractor  *seo.Extractor
	linkChecker   *checker.LinkChecker
	assetChecker  *checker.AssetChecker
	reportBuilder *report.Builder
	maxDepth      int
}

// Run запускает процесс обхода
func (c *Crawler) Run(ctx context.Context) {
	for ctx.Err() == nil {
		// Берём URL из очереди
		item := c.state.Queue.Dequeue()

		// Если очереди пуста, ждём завершения всех воркеров
		if item == nil {
			c.state.WG.Wait()

			// После завершения воркеров проверяем очередь снова
			// (воркеры могли добавить новые URL)
			if c.state.Queue.IsEmpty() {
				break
			}
			continue
		}

		// Обрабатываем URL
		c.processURLWithWorker(ctx, item.URL, item.Depth)
	}

	// Финальное ожидание завершения всех воркеров
	c.state.WG.Wait()
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
		return
	default:
	}

	if c.state.Visited.Contains(urlStr) {
		return
	}

	c.state.Visited.Add(urlStr)

	page := report.Page{
		URL:   urlStr,
		Depth: depth,
	}

	// Выполняем HTTP-запрос
	result := c.fetcher.Fetch(ctx, urlStr)
	page.HTTPStatus = result.StatusCode

	if result.Error != nil {
		page.Error = result.Error.Error()
		report.SetPageStatus(&page)
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		page.SEO = &seo.SEO{}
		page.BrokenLinks = []checker.BrokenLink{}
		page.Assets = []checker.Asset{}
		c.reportBuilder.AddPage(page)
		return
	}

	report.SetPageStatus(&page)

	if result.HTMLContent != "" {
		pageURL, _ := url.Parse(urlStr)

		// Извлекаем SEO данные
		page.SEO = c.seoExtractor.Extract(result.HTMLContent)

		// Извлекаем ссылки
		links := c.parser.ExtractLinks(result.HTMLContent, pageURL)

		// Проверяем битые ссылки
		page.BrokenLinks, page.DiscoveredAt = c.linkChecker.CheckLinks(ctx, links)

		// Проверяем ассеты
		page.Assets = c.assetChecker.CheckAssets(ctx, result.HTMLContent, pageURL)

		// Добавляем внутренние ссылки в очередь
		// depth+1 < maxDepth означает: следующий уровень не превысит maxDepth
		if depth+1 < c.maxDepth && page.Status == "ok" {
			c.enqueueInternalLinks(links, depth+1)
		}
	} else {
		// Если нет HTML контента, но запрос был успешен
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		page.SEO = &seo.SEO{}
		page.BrokenLinks = []checker.BrokenLink{}
		page.Assets = []checker.Asset{}
	}

	c.reportBuilder.AddPage(page)
}

// enqueueInternalLinks добавляет внутренние ссылки в очередь
func (c *Crawler) enqueueInternalLinks(links []string, depth int) {
	toAdd := []state.URLWithDepth{}

	for _, link := range links {
		linkURL, err := url.Parse(link)
		if err != nil || !urlutil.IsSameDomain(linkURL, c.state.BaseURL) {
			continue
		}

		// Нормализуем URL перед добавлением
		normalized := urlutil.NormalizeURL(linkURL)
		if !c.state.Visited.Contains(normalized) {
			toAdd = append(toAdd, state.URLWithDepth{URL: normalized, Depth: depth})
		}
	}

	if len(toAdd) > 0 {
		c.state.Queue.Enqueue(toAdd)
	}
}
