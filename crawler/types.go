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
	RootURL   string    `json:"root_url"`
	Depth     int       `json:"depth"`
	GeneratedAt string  `json:"generated_at"`
	Pages     []Page    `json:"pages"`
}

// Page содержит информацию о странице
type Page struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

