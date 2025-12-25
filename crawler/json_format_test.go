package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Тест 1: Проверка структуры JSON с эталоном
func TestJSONFormat_MatchesReference(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			html := `
                <html>
                <head>
                    <title>Example title</title>
                    <meta name="description" content="Example description">
                </head>
                <body>
                    <h1>Heading</h1>
                    <a href="/missing">Broken link</a>
                    <img src="/static/logo.png">
                </body>
                </html>
            `

			switch req.URL.Path {
			case "/":
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader(html)),
					Header:        http.Header{"Content-Type": []string{"text/html"}},
					ContentLength: int64(len(html)),
				}, nil
			case "/missing":
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			case "/static/logo.png":
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader("")),
					ContentLength: 12345,
				}, nil
			default:
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
		},
	}

	opts := Options{
		URL:         "https://example.com",
		Depth:       0,
		Concurrency: 1,
		Timeout:     5 * time.Second,
		HTTPClient:  mockClient,
		IndentJSON:  false,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Парсим результат
	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Проверяем обязательные поля верхнего уровня
	if report.RootURL == "" {
		t.Error("root_url should not be empty")
	}
	if report.GeneratedAt == "" {
		t.Error("generated_at should not be empty")
	}
	if report.Pages == nil {
		t.Error("pages should not be nil")
	}

	// Проверяем что есть хотя бы одна страница
	if len(report.Pages) == 0 {
		t.Fatal("Expected at least one page")
	}

	page := report.Pages[0]

	// Проверяем все обязательные поля Page
	requiredPageFields := map[string]bool{
		"url":           page.URL != "",
		"http_status":   page.HTTPStatus != 0,
		"status":        page.Status != "",
		"discovered_at": page.DiscoveredAt != "",
	}

	for field, present := range requiredPageFields {
		if !present {
			t.Errorf("Page field '%s' is missing or empty", field)
		}
	}

	// Error может быть пустой строкой, но должен присутствовать в JSON
	var rawPage map[string]interface{}
	pageJSON, _ := json.Marshal(page)
	json.Unmarshal(pageJSON, &rawPage)

	requiredKeys := []string{"url", "depth", "http_status", "status", "seo", "broken_links", "assets", "discovered_at"}
	for _, key := range requiredKeys {
		if _, exists := rawPage[key]; !exists {
			t.Errorf("Required key '%s' is missing from JSON", key)
		}
	}
}

// Тест 2: Проверка что все ключи присутствуют даже при пустых значениях
func TestJSONFormat_AllKeysPresent(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html><body>No SEO</body></html>")),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		},
	}

	opts := Options{
		URL:         "https://example.com",
		Depth:       0,
		Concurrency: 1,
		Timeout:     5 * time.Second,
		HTTPClient:  mockClient,
		IndentJSON:  false,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Парсим в map для проверки ключей
	var raw map[string]interface{}
	if err := json.Unmarshal(result, &raw); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Проверяем обязательные ключи верхнего уровня
	requiredTopKeys := []string{"root_url", "depth", "generated_at", "pages"}
	for _, key := range requiredTopKeys {
		if _, exists := raw[key]; !exists {
			t.Errorf("Required top-level key '%s' is missing", key)
		}
	}

	// Проверяем ключи в первой странице
	pages := raw["pages"].([]interface{})
	if len(pages) == 0 {
		t.Fatal("No pages in report")
	}

	page := pages[0].(map[string]interface{})
	requiredPageKeys := []string{"url", "depth", "http_status", "status", "seo", "broken_links", "assets", "discovered_at"}

	for _, key := range requiredPageKeys {
		if _, exists := page[key]; !exists {
			t.Errorf("Required page key '%s' is missing", key)
		}
	}

	// Проверяем что пустые массивы - это [], а не null
	if page["broken_links"] == nil {
		t.Error("broken_links should be [] not null")
	}
	if page["assets"] == nil {
		t.Error("assets should be [] not null")
	}
}

// Тест 3: Проверка IndentJSON - содержимое одинаковое
func TestJSONFormat_IndentPreservesContent(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html><body>Test</body></html>")),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		},
	}

	opts := Options{
		URL:         "https://example.com",
		Depth:       0,
		Concurrency: 1,
		Timeout:     5 * time.Second,
		HTTPClient:  mockClient,
	}

	// Без отступов
	opts.IndentJSON = false
	compactResult, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// С отступами
	opts.IndentJSON = true
	indentedResult, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Парсим оба результата
	var compactReport, indentedReport Report
	if err := json.Unmarshal(compactResult, &compactReport); err != nil {
		t.Fatalf("Failed to parse compact JSON: %v", err)
	}
	if err := json.Unmarshal(indentedResult, &indentedReport); err != nil {
		t.Fatalf("Failed to parse indented JSON: %v", err)
	}

	// Сравниваем содержимое
	compactReparse, _ := json.Marshal(compactReport)
	indentedReparse, _ := json.Marshal(indentedReport)

	if string(compactReparse) != string(indentedReparse) {
		t.Error("Content differs between compact and indented JSON")
	}

	// Проверяем что indented длиннее (есть пробелы и переводы строк)
	if len(indentedResult) <= len(compactResult) {
		t.Error("Indented JSON should be longer than compact")
	}
}

// Тест 4: Проверка формата времени ISO8601
func TestJSONFormat_ISO8601Timestamps(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		},
	}

	opts := Options{
		URL:         "https://example.com",
		Depth:       0,
		Concurrency: 1,
		Timeout:     5 * time.Second,
		HTTPClient:  mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	json.Unmarshal(result, &report)

	// Проверяем формат generated_at
	_, err = time.Parse(time.RFC3339, report.GeneratedAt)
	if err != nil {
		t.Errorf("generated_at is not valid ISO8601: %v", err)
	}

	// Проверяем формат discovered_at
	if len(report.Pages) > 0 && report.Pages[0].DiscoveredAt != "" {
		_, err = time.Parse(time.RFC3339, report.Pages[0].DiscoveredAt)
		if err != nil {
			t.Errorf("discovered_at is not valid ISO8601: %v", err)
		}
	}
}

// Тест 5: Проверка что пустые массивы - это [], а не null
func TestJSONFormat_EmptyArraysNotNull(t *testing.T) {
	page := Page{
		URL:          "https://example.com",
		Depth:        0,
		HTTPStatus:   200,
		Status:       "ok",
		Error:        "",
		BrokenLinks:  []BrokenLink{},
		Assets:       []Asset{},
		DiscoveredAt: "2024-06-01T12:00:00Z",
	}

	jsonData, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(jsonData)

	// Проверяем что массивы - это [], а не null
	if !strings.Contains(jsonStr, `"broken_links":[]`) {
		t.Error("broken_links should be [] for empty array")
	}
	if !strings.Contains(jsonStr, `"assets":[]`) {
		t.Error("assets should be [] for empty array")
	}
}
