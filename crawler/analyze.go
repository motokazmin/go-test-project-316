package crawler

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Analyze анализирует структуру веб-сайта
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	// 1. Нормализуем опции
	normalizeOptions(&opts)

	// 2. Валидируем URL
	rootURL, err := ParseAndValidateURL(opts.URL)
	if err != nil {
		return nil, err
	}

	// 3. Создаем компоненты
	rateLimiter := NewRateLimiter(ctx, opts.Delay)
	state := NewCrawlState(rootURL, opts.Workers, rateLimiter)
	fetcher := NewFetcher(opts, rateLimiter)
	parser := NewHTMLParser()
	seoExtractor := NewSEOExtractor()
	linkChecker := NewLinkChecker(fetcher, opts.Workers)
	reportBuilder := NewReportBuilder(rootURL, opts.Depth)

	// 4. Создаем crawler
	crawler := &Crawler{
		state:         state,
		fetcher:       fetcher,
		parser:        parser,
		seoExtractor:  seoExtractor,
		linkChecker:   linkChecker,
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
	state         *CrawlState
	fetcher       *Fetcher
	parser        *HTMLParser
	seoExtractor  *SEOExtractor
	linkChecker   *LinkChecker
	reportBuilder *ReportBuilder
	maxDepth      int
}

// Run запускает процесс обхода
func (c *Crawler) Run(ctx context.Context) {
	for !c.state.Queue.IsEmpty() {
		if ctx.Err() != nil {
			break
		}

		item := c.state.Queue.Dequeue()
		if item == nil {
			break
		}

		c.processURLWithWorker(ctx, item.url, item.depth)
	}

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

	page := Page{
		URL:   urlStr,
		Depth: depth,
	}

	// Выполняем HTTP-запрос
	result := c.fetcher.Fetch(ctx, urlStr)
	page.HTTPStatus = result.StatusCode

	if result.Error != nil {
		page.Error = result.Error.Error()
		SetPageStatus(&page)
		c.reportBuilder.AddPage(page)
		return
	}

	SetPageStatus(&page)

	if result.HTMLContent != "" {
		// Извлекаем SEO данные
		page.SEO = c.seoExtractor.Extract(result.HTMLContent)

		// Извлекаем ссылки
		pageURL, _ := url.Parse(urlStr)
		links := c.parser.ExtractLinks(result.HTMLContent, pageURL)

		// Проверяем битые ссылки
		page.BrokenLinks, page.DiscoveredAt = c.linkChecker.CheckLinks(ctx, links)

		// Добавляем внутренние ссылки в очередь
		if depth < c.maxDepth && page.Status == "ok" {
			c.enqueueInternalLinks(links, depth+1)
		}
	}

	c.reportBuilder.AddPage(page)
}

// enqueueInternalLinks добавляет внутренние ссылки в очередь
func (c *Crawler) enqueueInternalLinks(links []string, depth int) {
	toAdd := []urlWithDepth{}

	for _, link := range links {
		linkURL, err := url.Parse(link)
		if err != nil || !IsSameDomain(linkURL, c.state.BaseURL) {
			continue
		}

		normalized := linkURL.String()
		if !c.state.Visited.Contains(normalized) {
			toAdd = append(toAdd, urlWithDepth{url: normalized, depth: depth})
		}
	}

	if len(toAdd) > 0 {
		c.state.Queue.Enqueue(toAdd)
	}
}

// normalizeOptions устанавливает значения по умолчанию
func normalizeOptions(opts *Options) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
}
