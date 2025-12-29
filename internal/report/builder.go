package report

import (
	"encoding/json"
	"net/url"
	"sort"
	"sync"
	"time"

	"code/internal/checker"
	"code/internal/seo"
)

// Page содержит информацию о странице
type Page struct {
	URL          string               `json:"url"`
	Depth        int                  `json:"depth"`
	HTTPStatus   int                  `json:"http_status"`
	Status       string               `json:"status"`
	Error        string               `json:"error,omitempty"`
	BrokenLinks  []checker.BrokenLink `json:"broken_links"`
	DiscoveredAt string               `json:"discovered_at"`
	SEO          *seo.SEO             `json:"seo"`
	Assets       []checker.Asset      `json:"assets"`
}

// Report содержит результат обхода сайта
type Report struct {
	RootURL     string `json:"root_url"`
	Depth       int    `json:"depth"`
	GeneratedAt string `json:"generated_at"`
	Pages       []Page `json:"pages"`
}

// Builder создает и управляет отчетом
type Builder struct {
	report *Report
	mu     sync.Mutex
}

// NewBuilder создает новый builder
func NewBuilder(rootURL *url.URL, depth int) *Builder {
	return &Builder{
		report: &Report{
			RootURL:     rootURL.String(),
			Depth:       depth,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Pages:       []Page{},
		},
	}
}

// AddPage добавляет страницу в отчет (потокобезопасно)
func (rb *Builder) AddPage(page Page) {
	// Для страниц с ошибками оставляем nil
	if page.Error != "" {
		page.BrokenLinks = nil
		page.Assets = nil
	} else {
		// Для успешных страниц инициализируем пустыми массивами
		if page.BrokenLinks == nil {
			page.BrokenLinks = []checker.BrokenLink{}
		}
		if page.Assets == nil {
			page.Assets = []checker.Asset{}
		}
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.report.Pages = append(rb.report.Pages, page)
}

// Encode кодирует отчет в JSON
func (rb *Builder) Encode(indent bool) ([]byte, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Сортируем страницы по URL для детерминированного порядка
	sort.SliceStable(rb.report.Pages, func(i, j int) bool {
		return rb.report.Pages[i].URL < rb.report.Pages[j].URL
	})

	if indent {
		return json.MarshalIndent(rb.report, "", "  ")
	}
	return json.Marshal(rb.report)
}

// SetPageStatus устанавливает статус страницы по HTTP коду
func SetPageStatus(page *Page) {
	if page.HTTPStatus == 0 {
		page.Status = "error"
		if page.Error == "" {
			page.Error = "no response received"
		}
		return
	}

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
