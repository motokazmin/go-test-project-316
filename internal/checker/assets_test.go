package checker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"code/internal/httputil"
	"code/internal/parser"
)

// MockHTTPClient для подмены реальных HTTP запросов
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// Тест 1: Базовая проверка ассета с Content-Length
func TestAssetChecker_WithContentLength(t *testing.T) {
	callCount := 0
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode:    200,
				ContentLength: 12345,
				Body:          io.NopCloser(strings.NewReader("")),
				Header:        http.Header{},
			}, nil
		},
	}

	cfg := httputil.FetcherConfig{
		Client:  mockClient,
		Timeout: 5 * time.Second,
	}

	fetcher := httputil.NewFetcher(cfg, nil)
	htmlParser := parser.NewHTMLParser()
	checker := NewAssetChecker(fetcher, htmlParser, 4)

	result := checker.fetchAsset(context.Background(), "https://example.com/logo.png")

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("Expected status 200, got: %d", result.StatusCode)
	}
	if result.SizeBytes != 12345 {
		t.Errorf("Expected size 12345, got: %d", result.SizeBytes)
	}
}

// Тест 2: Ассет без Content-Length (размер из тела)
func TestAssetChecker_WithoutContentLength(t *testing.T) {
	body := strings.Repeat("x", 5000)

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode:    200,
				ContentLength: -1, // Отсутствует
				Body:          io.NopCloser(strings.NewReader(body)),
				Header:        http.Header{},
			}, nil
		},
	}

	cfg := httputil.FetcherConfig{
		Client:  mockClient,
		Timeout: 5 * time.Second,
	}

	fetcher := httputil.NewFetcher(cfg, nil)
	htmlParser := parser.NewHTMLParser()
	checker := NewAssetChecker(fetcher, htmlParser, 4)

	result := checker.fetchAsset(context.Background(), "https://example.com/script.js")

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if result.SizeBytes != 5000 {
		t.Errorf("Expected size 5000 (from body), got: %d", result.SizeBytes)
	}
}

// Тест 3: Ассет с ошибкой 404
func TestAssetChecker_NotFound(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 404,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		},
	}

	cfg := httputil.FetcherConfig{
		Client:  mockClient,
		Timeout: 5 * time.Second,
	}

	fetcher := httputil.NewFetcher(cfg, nil)
	htmlParser := parser.NewHTMLParser()
	checker := NewAssetChecker(fetcher, htmlParser, 4)

	result := checker.fetchAsset(context.Background(), "https://example.com/missing.png")

	if result.StatusCode != 404 {
		t.Errorf("Expected status 404, got: %d", result.StatusCode)
	}
	if result.Error == nil {
		t.Error("Expected error for 404 status")
	}
}

// Тест 4: Сетевая ошибка
func TestAssetChecker_NetworkError(t *testing.T) {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("connection timeout")
		},
	}

	cfg := httputil.FetcherConfig{
		Client:  mockClient,
		Timeout: 5 * time.Second,
	}

	fetcher := httputil.NewFetcher(cfg, nil)
	htmlParser := parser.NewHTMLParser()
	checker := NewAssetChecker(fetcher, htmlParser, 4)

	result := checker.fetchAsset(context.Background(), "https://example.com/logo.png")

	if result.Error == nil {
		t.Error("Expected network error")
	}
	if result.StatusCode != 0 {
		t.Errorf("Expected status 0 for network error, got: %d", result.StatusCode)
	}
}

// Тест 5: Извлечение ассетов из HTML
func TestHTMLParser_ExtractAssets(t *testing.T) {
	html := `
        <html>
        <head>
            <link rel="stylesheet" href="/style.css">
            <script src="/app.js"></script>
        </head>
        <body>
            <img src="/logo.png" alt="Logo">
            <img src="https://cdn.example.com/photo.jpg">
            <script src="https://cdn.example.com/analytics.js"></script>
        </body>
        </html>
    `

	htmlParser := parser.NewHTMLParser()
	pageURL, _ := url.Parse("https://example.com/page")

	assets := htmlParser.ExtractAssets(html, pageURL)

	// Проверяем количество
	expectedCount := 5
	if len(assets) != expectedCount {
		t.Errorf("Expected %d assets, got: %d", expectedCount, len(assets))
	}

	// Проверяем типы
	imageCount := 0
	scriptCount := 0
	styleCount := 0

	for _, asset := range assets {
		switch asset.AssetType {
		case "image":
			imageCount++
		case "script":
			scriptCount++
		case "style":
			styleCount++
		}
	}

	if imageCount != 2 {
		t.Errorf("Expected 2 images, got: %d", imageCount)
	}
	if scriptCount != 2 {
		t.Errorf("Expected 2 scripts, got: %d", scriptCount)
	}
	if styleCount != 1 {
		t.Errorf("Expected 1 style, got: %d", styleCount)
	}
}
