package crawler

import (
	"net/http"
	"sync"
	"time"
)

// HTTPClient интерфейс для выполнения HTTP запросов
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Options содержит параметры для обхода сайта
type Options struct {
	URL        string
	Depth      int
	Retries    int
	Delay      time.Duration
	Timeout    time.Duration
	UserAgent  string
	Workers    int
	IndentJSON bool
	HTTPClient HTTPClient
}

// Report содержит результат обхода сайта
type Report struct {
	RootURL     string `json:"root_url"`
	Depth       int    `json:"depth"`
	GeneratedAt string `json:"generated_at"`
	Pages       []Page `json:"pages"`
	pagesMutex  sync.Mutex
}

// BrokenLink содержит информацию о битой ссылке
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SEO содержит базовые SEO параметры страницы
type SEO struct {
	HasTitle       bool    `json:"has_title"`
	Title          *string `json:"title"`
	HasDescription bool    `json:"has_description"`
	Description    *string `json:"description"`
	HasH1          bool    `json:"has_h1"`
}

// Page содержит информацию о странице
type Page struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []BrokenLink `json:"broken_links,omitempty"`
	DiscoveredAt string       `json:"discovered_at,omitempty"`
	SEO          *SEO         `json:"seo,omitempty"`
}
