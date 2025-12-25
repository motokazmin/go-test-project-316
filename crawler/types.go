package crawler

import (
	"net/http"
	"time"
)

// HTTPClient интерфейс для выполнения HTTP запросов
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

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

// Report содержит результат обхода сайта
type Report struct {
	RootURL     string `json:"root_url"`
	Depth       int    `json:"depth"`
	GeneratedAt string `json:"generated_at"`
	Pages       []Page `json:"pages"`
}

// BrokenLink содержит информацию о битой ссылке
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

// SEO содержит базовые SEO параметры страницы
type SEO struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
	H1             string `json:"h1,omitempty"`
}

// Page содержит информацию о странице
type Page struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []BrokenLink `json:"broken_links"`
	DiscoveredAt string       `json:"discovered_at"`
	SEO          *SEO         `json:"seo"`
	Assets       []Asset      `json:"assets"`
}

// Asset содержит информацию об ассете (картинка, скрипт, стиль)
type Asset struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error"`
}

// AssetResult содержит результат проверки ассета
type AssetResult struct {
	URL        string
	Type       string
	StatusCode int
	SizeBytes  int64
	Error      error
}

// ========== Внутренние типы для управления состоянием ==========

// urlWithDepth представляет URL с его глубиной в дереве обхода
type urlWithDepth struct {
	url   string
	depth int
}

// FetchResult содержит результат HTTP-запроса
type FetchResult struct {
	StatusCode  int
	HTMLContent string
	Error       error
}
