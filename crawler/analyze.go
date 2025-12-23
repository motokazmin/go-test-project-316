package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// urlWithDepth представляет URL с его глубиной в дереве обхода
type urlWithDepth struct {
	url   string
	depth int
}

// CrawlState содержит состояние процесса краулирования
type CrawlState struct {
	visited      map[string]bool
	visitedMutex sync.Mutex
	queue        []urlWithDepth
	queueMutex   sync.Mutex
	semaphore    chan struct{}
	wg           sync.WaitGroup
}

// Analyze анализирует структуру веб-сайта
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	normalizeOptions(&opts)

	rootURL, err := parseAndValidateURL(opts.URL)
	if err != nil {
		return nil, err
	}

	report := createReport(rootURL, opts.Depth)
	state := initializeCrawlState(rootURL, opts.Workers)

	processQueueWithWorkers(ctx, opts, report, state)

	return encodeReport(report, opts.IndentJSON)
}

// normalizeOptions устанавливает значения по умолчанию для опций
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

// parseAndValidateURL нормализует и парсит URL
func parseAndValidateURL(urlStr string) (*url.URL, error) {
	normalized := normalizeURLString(urlStr)

	parsedURL, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL: no host")
	}

	return parsedURL, nil
}

// normalizeURLString добавляет схему если её нет
func normalizeURLString(urlStr string) string {
	if !strings.Contains(urlStr, "://") {
		return "https://" + urlStr
	}
	return urlStr
}

// createReport инициализирует новый отчет
func createReport(rootURL *url.URL, depth int) *Report {
	return &Report{
		RootURL:     rootURL.String(),
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Pages:       []Page{},
	}
}

// initializeCrawlState инициализирует состояние краулирования
func initializeCrawlState(rootURL *url.URL, workers int) *CrawlState {
	return &CrawlState{
		visited:   make(map[string]bool),
		queue:     []urlWithDepth{{url: rootURL.String(), depth: 0}},
		semaphore: make(chan struct{}, workers),
	}
}

// processQueueWithWorkers обрабатывает очередь URL'ов с ограничением по worker'ам
func processQueueWithWorkers(ctx context.Context, opts Options, report *Report, state *CrawlState) {
	for len(state.queue) > 0 {
		if isContextDone(ctx) {
			return
		}

		item := dequeueURL(state)
		if item == nil {
			break
		}

		processURLWithWorker(ctx, opts, report, state, item.url, item.depth)

		applyDelay(opts.Delay)
	}

	state.wg.Wait()
}

// isContextDone проверяет отменен ли контекст
func isContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// dequeueURL извлекает следующий URL из очереди (потокобезопасно)
func dequeueURL(state *CrawlState) *urlWithDepth {
	state.queueMutex.Lock()
	defer state.queueMutex.Unlock()

	if len(state.queue) == 0 {
		return nil
	}

	item := state.queue[0]
	state.queue = state.queue[1:]
	return &item
}

// processURLWithWorker обрабатывает URL в отдельном worker'е
func processURLWithWorker(ctx context.Context, opts Options, report *Report, state *CrawlState, urlStr string, depth int) {
	state.wg.Add(1)
	state.semaphore <- struct{}{}

	go func() {
		defer state.wg.Done()
		defer func() { <-state.semaphore }()

		processSingleURL(ctx, opts, report, state, urlStr, depth)
	}()
}

// processSingleURL обрабатывает один URL
func processSingleURL(ctx context.Context, opts Options, report *Report, state *CrawlState, urlStr string, depth int) {
	if isAlreadyVisited(state, urlStr) {
		return
	}

	markAsVisited(state, urlStr)

	page := createPageRecord(urlStr, depth)

	fetchAndUpdatePage(ctx, opts, page)

	addPageToReport(state, report, page)
}

// isAlreadyVisited проверяет был ли URL уже посещен (потокобезопасно)
func isAlreadyVisited(state *CrawlState, urlStr string) bool {
	state.visitedMutex.Lock()
	defer state.visitedMutex.Unlock()

	return state.visited[urlStr]
}

// markAsVisited отмечает URL как посещенный (потокобезопасно)
func markAsVisited(state *CrawlState, urlStr string) {
	state.visitedMutex.Lock()
	defer state.visitedMutex.Unlock()

	state.visited[urlStr] = true
}

// createPageRecord создает запись о странице
func createPageRecord(urlStr string, depth int) *Page {
	return &Page{
		URL:   urlStr,
		Depth: depth,
	}
}

// fetchAndUpdatePage выполняет HTTP запрос и обновляет данные страницы
func fetchAndUpdatePage(ctx context.Context, opts Options, page *Page) {
	fetchWithRetries(ctx, opts, page)

	if page.HTTPStatus != 0 {
		setStatusFromHTTPCode(page)
	} else if page.Error == "" {
		page.Status = "error"
		page.Error = "no response received"
	}
}

// fetchWithRetries выполняет запрос с повторами
func fetchWithRetries(ctx context.Context, opts Options, page *Page) {
	var lastErr error

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		if attempt > 0 {
			if !waitForRetry(ctx) {
				return
			}
		}

		err := performHTTPRequest(ctx, opts, page)
		if err == nil {
			lastErr = nil
			break
		}

		lastErr = err
	}

	if lastErr != nil {
		page.Status = "error"
		page.Error = lastErr.Error()
	}
}

// waitForRetry ждет перед повтором и проверяет контекст
func waitForRetry(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(100 * time.Millisecond):
		return true
	}
}

// performHTTPRequest выполняет один HTTP запрос
func performHTTPRequest(ctx context.Context, opts Options, page *Page) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, page.URL, nil)
	if err != nil {
		return err
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	page.HTTPStatus = resp.StatusCode

	// Если статус OK и это HTML, парсим содержимое
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if isHTMLContent(resp.Header.Get("Content-Type")) {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				htmlContent := string(body)
				// Проверяем битые ссылки
				checkBrokenLinks(ctx, opts, page, htmlContent)
				// Извлекаем SEO параметры
				page.SEO = extractSEOData(htmlContent)
			}
		}
	}

	return nil
}

// isHTMLContent проверяет является ли контент HTML
func isHTMLContent(contentType string) bool {
	return strings.Contains(contentType, "text/html")
}

// setStatusFromHTTPCode определяет статус по HTTP коду
func setStatusFromHTTPCode(page *Page) {
	switch {
	case page.HTTPStatus >= 200 && page.HTTPStatus < 300:
		page.Status = "ok"
	case page.HTTPStatus >= 300 && page.HTTPStatus < 400:
		page.Status = "redirect"
	case page.HTTPStatus >= 400 && page.HTTPStatus < 500:
		page.Status = "client_error"
	default:
		page.Status = "server_error"
	}
}

// addPageToReport добавляет страницу в отчет (потокобезопасно)
func addPageToReport(state *CrawlState, report *Report, page *Page) {
	state.visitedMutex.Lock()
	defer state.visitedMutex.Unlock()

	report.Pages = append(report.Pages, *page)
}

// applyDelay применяет задержку если она установлена
func applyDelay(delay time.Duration) {
	if delay > 0 {
		time.Sleep(delay)
	}
}

// extractLinksFromHTML извлекает все ссылки из HTML содержимого
func extractLinksFromHTML(htmlContent string, pageURL *url.URL) []string {
	links := []string{}
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					link := resolveURL(attr.Val, pageURL)
					if link != "" {
						links = append(links, link)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return links
}

// resolveURL преобразует относительный URL в абсолютный
func resolveURL(href string, baseURL *url.URL) string {
	if href == "" {
		return ""
	}

	// Игнорируем якоря, javascript и mailto
	if strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") {
		return ""
	}

	parsedURL, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// Проверяем схему
	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ""
	}

	resolvedURL := baseURL.ResolveReference(parsedURL)
	return resolvedURL.String()
}

// checkBrokenLinks проверяет доступность ссылок на странице (параллельно)
func checkBrokenLinks(ctx context.Context, opts Options, page *Page, htmlContent string) {
	pageURL, err := url.Parse(page.URL)
	if err != nil {
		return
	}

	links := extractLinksFromHTML(htmlContent, pageURL)
	if len(links) == 0 {
		page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
		return
	}

	// Параллельная проверка ссылок с ограничением по worker'ам
	semaphore := make(chan struct{}, opts.Workers)
	resultChan := make(chan BrokenLink, len(links))
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(linkURL string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if isBrokenLink(ctx, opts, linkURL) {
				brokenLink := checkLink(ctx, opts, linkURL)
				resultChan <- brokenLink
			}
		}(link)
	}

	// Закрываем resultChan когда все goroutine'ы завершены
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Собираем результаты
	brokenLinks := []BrokenLink{}
	for brokenLink := range resultChan {
		brokenLinks = append(brokenLinks, brokenLink)
	}

	page.BrokenLinks = brokenLinks
	page.DiscoveredAt = time.Now().UTC().Format(time.RFC3339)
}

// isBrokenLink проверяет является ли ссылка битой (быстрая проверка)
func isBrokenLink(ctx context.Context, opts Options, linkURL string) bool {
	statusCode, _ := checkLinkStatus(ctx, opts, linkURL)
	return statusCode >= 400
}

// checkLink проверяет доступность ссылки и возвращает информацию об ошибке
func checkLink(ctx context.Context, opts Options, linkURL string) BrokenLink {
	statusCode, errMsg := checkLinkStatus(ctx, opts, linkURL)

	brokenLink := BrokenLink{URL: linkURL}
	if errMsg != "" {
		brokenLink.Error = errMsg
	} else if statusCode >= 400 {
		brokenLink.StatusCode = statusCode
	}

	return brokenLink
}

// checkLinkStatus проверяет статус ссылки
func checkLinkStatus(ctx context.Context, opts Options, linkURL string) (int, string) {
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodHead, linkURL, nil)
	if err != nil {
		// Если HEAD не сработал, пробуем GET
		req, err = http.NewRequestWithContext(timeoutCtx, http.MethodGet, linkURL, nil)
		if err != nil {
			return 0, err.Error()
		}
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()

	// Для GET запроса читаем тело чтобы освободить соединение
	if req.Method == http.MethodGet {
		io.ReadAll(resp.Body)
	}

	return resp.StatusCode, ""
}

// extractSEOData извлекает SEO параметры из HTML содержимого
func extractSEOData(htmlContent string) *SEO {
	seo := &SEO{}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return seo
	}

	// Ищем title
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if !seo.HasTitle && n.Type == html.ElementNode && n.Data == "title" {
			seo.HasTitle = true
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				title := strings.TrimSpace(n.FirstChild.Data)
				seo.Title = &title
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}

	// Ищем meta description и h1
	var findMetaAndHeadings func(*html.Node)
	findMetaAndHeadings = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if !seo.HasDescription && n.Data == "meta" {
				name := ""
				content := ""
				for _, attr := range n.Attr {
					if attr.Key == "name" && attr.Val == "description" {
						name = "description"
					}
					if attr.Key == "content" {
						content = attr.Val
					}
				}
				if name == "description" {
					seo.HasDescription = true
					seo.Description = &content
				}
			}

			if !seo.HasH1 && n.Data == "h1" {
				text := extractTextContent(n)
				seo.HasH1 = text != ""
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findMetaAndHeadings(c)
		}
	}

	findTitle(doc)
	findMetaAndHeadings(doc)

	return seo
}

// extractTextContent рекурсивно извлекает текстовое содержимое узла
func extractTextContent(n *html.Node) string {
	if n == nil {
		return ""
	}

	var text strings.Builder

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(strings.TrimSpace(node.Data))
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(n)
	return strings.TrimSpace(text.String())
}

// encodeReport кодирует отчет в JSON
func encodeReport(report *Report, indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(report, "", "  ")
	}
	return json.Marshal(report)
}
