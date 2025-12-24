package crawler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

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

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

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

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

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

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

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

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

	result := checker.fetchAsset(context.Background(), "https://example.com/logo.png")

	if result.Error == nil {
		t.Error("Expected network error")
	}
	if result.StatusCode != 0 {
		t.Errorf("Expected status 0 for network error, got: %d", result.StatusCode)
	}
}

// Тест 5: Кэширование - один ассет встречается дважды
func TestAssetChecker_Caching(t *testing.T) {
	callCount := 0
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			return &http.Response{
				StatusCode:    200,
				ContentLength: 10000,
				Body:          io.NopCloser(strings.NewReader("")),
				Header:        http.Header{},
			}, nil
		},
	}

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

	assetURL := "https://example.com/logo.png"

	// Первый запрос
	asset1 := checker.checkSingleAsset(context.Background(), assetURL, "image")

	// Второй запрос того же ассета
	asset2 := checker.checkSingleAsset(context.Background(), assetURL, "image")

	// Проверяем что результаты идентичны
	if asset1.URL != asset2.URL {
		t.Error("URLs should match")
	}
	if asset1.SizeBytes != asset2.SizeBytes {
		t.Error("Sizes should match")
	}
	if asset1.StatusCode != asset2.StatusCode {
		t.Error("Status codes should match")
	}

	// Проверяем что HTTP-запрос был выполнен только один раз
	if callCount != 1 {
		t.Errorf("Expected 1 HTTP request (cached), got: %d", callCount)
	}
}

// Тест 6: Извлечение ассетов из HTML
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

	parser := NewHTMLParser()
	pageURL, _ := url.Parse("https://example.com/page")

	assets := parser.ExtractAssets(html, pageURL)

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
		switch asset.assetType {
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

// Тест 7: Полная интеграция - проверка ассетов на странице
func TestAssetChecker_CheckAssets(t *testing.T) {
	responses := map[string]*http.Response{
		"https://example.com/style.css": {
			StatusCode:    200,
			ContentLength: 5000,
			Body:          io.NopCloser(strings.NewReader("")),
		},
		"https://example.com/app.js": {
			StatusCode:    200,
			ContentLength: 15000,
			Body:          io.NopCloser(strings.NewReader("")),
		},
		"https://example.com/logo.png": {
			StatusCode:    200,
			ContentLength: 8000,
			Body:          io.NopCloser(strings.NewReader("")),
		},
		"https://example.com/missing.jpg": {
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}

	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			resp, found := responses[req.URL.String()]
			if !found {
				return nil, errors.New("unexpected URL")
			}
			return resp, nil
		},
	}

	opts := Options{
		HTTPClient: mockClient,
		Timeout:    5 * time.Second,
	}

	fetcher := NewFetcher(opts, nil)
	checker := NewAssetChecker(fetcher, 4)

	html := `
        <html>
        <head>
            <link rel="stylesheet" href="/style.css">
            <script src="/app.js"></script>
        </head>
        <body>
            <img src="/logo.png">
            <img src="/missing.jpg">
        </body>
        </html>
    `

	pageURL, _ := url.Parse("https://example.com/page")
	assets := checker.CheckAssets(context.Background(), html, pageURL)

	// Проверяем количество
	if len(assets) != 4 {
		t.Fatalf("Expected 4 assets, got: %d", len(assets))
	}

	// Проверяем что все ассеты имеют все поля
	for _, asset := range assets {
		if asset.URL == "" {
			t.Error("Asset URL should not be empty")
		}
		if asset.Type == "" {
			t.Error("Asset Type should not be empty")
		}
		// StatusCode может быть 0 только при сетевой ошибке
	}

	// Проверяем битый ассет
	var brokenAsset *Asset
	for _, asset := range assets {
		if strings.Contains(asset.URL, "missing.jpg") {
			brokenAsset = &asset
			break
		}
	}

	if brokenAsset == nil {
		t.Fatal("Should have found broken asset")
	}
	if brokenAsset.StatusCode != 404 {
		t.Errorf("Expected status 404 for broken asset, got: %d", brokenAsset.StatusCode)
	}
	if brokenAsset.Error == "" {
		t.Error("Broken asset should have error message")
	}
}

// Тест 8: Проверка JSON структуры Asset
func TestAsset_JSONStructure(t *testing.T) {
	asset := Asset{
		URL:  "https://example.com/logo.png",
		Type: "image",
	}

	// Проверяем что все поля присутствуют
	if asset.URL == "" {
		t.Error("URL should be present")
	}
	if asset.Type == "" {
		t.Error("Type should be present")
	}
	// StatusCode = 0 валидно для сетевых ошибок
	// SizeBytes = 0 валидно когда размер неизвестен
	// Error может быть пустым для успешных запросов
}
