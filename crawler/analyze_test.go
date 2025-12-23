package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// MockHTTPClient для подмены реальных HTTP запросов
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// TestAnalyzeBasic проверяет базовую функциональность
func TestAnalyzeBasic(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      1,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	if report.RootURL != "https://example.com" {
		t.Errorf("Expected root_url to be https://example.com, got %s", report.RootURL)
	}

	if report.Depth != 1 {
		t.Errorf("Expected depth to be 1, got %d", report.Depth)
	}

	if len(report.Pages) == 0 {
		t.Errorf("Expected at least one page in report")
	}

	page := report.Pages[0]
	if page.HTTPStatus != 200 {
		t.Errorf("Expected HTTP status 200, got %d", page.HTTPStatus)
	}

	if page.Status != "ok" {
		t.Errorf("Expected status 'ok', got %s", page.Status)
	}
}

// TestAnalyzeWithContext проверяет отмену через контекст
func TestAnalyzeWithContext(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			time.Sleep(100 * time.Millisecond)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Request:    req,
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Сразу отменяем контекст

	opts := Options{
		URL:        "https://example.com",
		Depth:      1,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(ctx, opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	// Отчет должен быть создан, даже если контекст отменен
	if report.RootURL == "" {
		t.Errorf("Expected root_url to be set")
	}
}

// TestAnalyzeErrorHandling проверяет обработку ошибок
func TestAnalyzeErrorHandling(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 404,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.HTTPStatus != 404 {
		t.Errorf("Expected HTTP status 404, got %d", page.HTTPStatus)
	}

	if page.Status != "client_error" {
		t.Errorf("Expected status 'client_error', got %s", page.Status)
	}
}

// TestAnalyzeDefaultOptions проверяет значения по умолчанию
func TestAnalyzeDefaultOptions(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	// Проверяем, что отчет содержит валидный timestamp
	if report.GeneratedAt == "" {
		t.Errorf("Expected generated_at to be set")
	}

	// Проверяем, что можем распарсить timestamp
	if _, err := time.Parse(time.RFC3339, report.GeneratedAt); err != nil {
		t.Errorf("Invalid generated_at format: %v", err)
	}
}

// TestAnalyzeURLNormalization проверяет нормализацию URL
func TestAnalyzeURLNormalization(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
				Request:    req,
			}, nil
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
	}

	for _, test := range tests {
		opts := Options{
			URL:        test.input,
			HTTPClient: mockClient,
		}

		result, err := Analyze(context.Background(), opts)
		if err != nil {
			t.Fatalf("Analyze failed for %s: %v", test.input, err)
		}

		var report Report
		if err := json.Unmarshal(result, &report); err != nil {
			t.Fatalf("Failed to unmarshal report: %v", err)
		}

		if report.RootURL != test.expected {
			t.Errorf("For input %s: expected %s, got %s", test.input, test.expected, report.RootURL)
		}
	}
}

// TestAnalyzeServerError проверяет обработку 500 ошибки
func TestAnalyzeServerError(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.HTTPStatus != 500 {
		t.Errorf("Expected HTTP status 500, got %d", page.HTTPStatus)
	}

	if page.Status != "server_error" {
		t.Errorf("Expected status 'server_error', got %s", page.Status)
	}
}

// TestAnalyzeRedirect проверяет обработку redirect
func TestAnalyzeRedirect(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 301,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.HTTPStatus != 301 {
		t.Errorf("Expected HTTP status 301, got %d", page.HTTPStatus)
	}

	if page.Status != "redirect" {
		t.Errorf("Expected status 'redirect', got %s", page.Status)
	}
}

// TestAnalyzeNetworkError проверяет обработку сетевой ошибки
func TestAnalyzeNetworkError(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.Status != "error" {
		t.Errorf("Expected status 'error', got %s", page.Status)
	}

	if page.Error == "" {
		t.Errorf("Expected error message, got empty")
	}
}

// TestAnalyzeTimeout проверяет обработку таймаута
func TestAnalyzeTimeout(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// Имитируем долгий запрос
			time.Sleep(100 * time.Millisecond)
			return nil, fmt.Errorf("context deadline exceeded")
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		Timeout:    10 * time.Millisecond,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.Status != "error" {
		t.Errorf("Expected status 'error' for timeout, got %s", page.Status)
	}
}

// TestAnalyzeRetries проверяет повторные попытки при ошибке
func TestAnalyzeRetries(t *testing.T) {
	attemptCount := 0
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			attemptCount++
			if attemptCount < 3 {
				return nil, fmt.Errorf("temporary error")
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		Retries:    2,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	if page.HTTPStatus != 200 {
		t.Errorf("Expected HTTP status 200 after retries, got %d", page.HTTPStatus)
	}

	if page.Status != "ok" {
		t.Errorf("Expected status 'ok' after retries, got %s", page.Status)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

// TestBrokenLinks проверяет обнаружение битых ссылок
func TestBrokenLinks(t *testing.T) {
	htmlContent := `<html>
		<body>
			<a href="/page">Good Link</a>
			<a href="/notfound">Broken Link</a>
		</body>
	</html>`

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path

			// Главная страница с содержимым HTML
			if path == "" || path == "/" {
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Body:       io.NopCloser(strings.NewReader(htmlContent)),
					Request:    req,
				}, nil
			}

			// Рабочая ссылка
			if path == "/page" {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			}

			// Битая ссылка
			if path == "/notfound" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    req,
				}, nil
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	if len(report.Pages) == 0 {
		t.Fatalf("Expected at least one page in report")
	}

	page := report.Pages[0]

	// Проверяем что найдена только битая ссылка
	if len(page.BrokenLinks) != 1 {
		t.Errorf("Expected 1 broken link, got %d", len(page.BrokenLinks))
	}

	if len(page.BrokenLinks) > 0 {
		brokenLink := page.BrokenLinks[0]
		if !strings.Contains(brokenLink.URL, "/notfound") {
			t.Errorf("Expected broken link to be /notfound, got %s", brokenLink.URL)
		}

		if brokenLink.StatusCode != 404 {
			t.Errorf("Expected status code 404, got %d", brokenLink.StatusCode)
		}
	}
}

// TestIgnoredLinks проверяет что якоря и javascript игнорируются
func TestIgnoredLinks(t *testing.T) {
	htmlContent := `<html>
		<body>
			<a href="#anchor">Anchor</a>
			<a href="javascript:void(0)">JavaScript</a>
			<a href="mailto:test@example.com">Email</a>
			<a href="">Empty</a>
		</body>
	</html>`

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader(htmlContent)),
				Request:    req,
			}, nil
		},
	}

	opts := Options{
		URL:        "https://example.com",
		Depth:      0,
		Workers:    1,
		HTTPClient: mockClient,
	}

	result, err := Analyze(context.Background(), opts)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	page := report.Pages[0]
	// Якоря, javascript, mailto и пустые ссылки должны быть проигнорированы
	if len(page.BrokenLinks) > 0 {
		t.Errorf("Expected no broken links for ignored patterns, got %d", len(page.BrokenLinks))
	}
}
