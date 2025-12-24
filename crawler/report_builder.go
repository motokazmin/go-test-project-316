package crawler

import (
	"encoding/json"
	"net/url"
	"sync"
	"time"
)

// ReportBuilder создает и управляет отчетом
type ReportBuilder struct {
	report *Report
	mu     sync.Mutex
}

// NewReportBuilder создает новый builder
func NewReportBuilder(rootURL *url.URL, depth int) *ReportBuilder {
	return &ReportBuilder{
		report: &Report{
			RootURL:     rootURL.String(),
			Depth:       depth,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Pages:       []Page{},
		},
	}
}

// AddPage добавляет страницу в отчет (потокобезопасно)
func (rb *ReportBuilder) AddPage(page Page) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.report.Pages = append(rb.report.Pages, page)
}

// Encode кодирует отчет в JSON
func (rb *ReportBuilder) Encode(indent bool) ([]byte, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

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
