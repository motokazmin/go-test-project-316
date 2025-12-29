package crawler

import (
	"net/http"
	"time"

	"code/internal/checker"
	"code/internal/httputil"
	"code/internal/report"
	"code/internal/seo"
)

// HTTPClient интерфейс для выполнения HTTP запросов
type HTTPClient = httputil.HTTPClient

// Options содержит параметры для обхода сайта
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  bool
	HTTPClient  HTTPClient
}

// Re-export public types from internal packages
type (
	// Report содержит результат обхода сайта
	Report = report.Report
	// Page содержит информацию о странице
	Page = report.Page
	// BrokenLink содержит информацию о битой ссылке
	BrokenLink = checker.BrokenLink
	// SEO содержит базовые SEO параметры страницы
	SEO = seo.SEO
	// Asset содержит информацию об ассете
	Asset = checker.Asset
)

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
