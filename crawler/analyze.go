package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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
	return nil
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

// encodeReport кодирует отчет в JSON
func encodeReport(report *Report, indent bool) ([]byte, error) {
	if indent {
		return json.MarshalIndent(report, "", "  ")
	}
	return json.Marshal(report)
}
