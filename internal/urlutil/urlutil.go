package urlutil

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseAndValidateURL парсит и валидирует URL
func ParseAndValidateURL(urlStr string) (*url.URL, error) {
	normalized := NormalizeURLString(urlStr)

	parsedURL, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL: no host")
	}

	return parsedURL, nil
}

// NormalizeURLString добавляет схему если её нет
func NormalizeURLString(urlStr string) string {
	if !strings.Contains(urlStr, "://") {
		return "https://" + urlStr
	}
	return urlStr
}

// NormalizeURL нормализует URL для избежания дубликатов
// Убирает trailing slash для корневого пути и fragment
func NormalizeURL(u *url.URL) string {
	// Клонируем URL чтобы не модифицировать оригинал
	normalized := *u

	// Убираем fragment (#section)
	normalized.Fragment = ""

	// Убираем trailing slash только для корневого пути
	if normalized.Path == "/" {
		normalized.Path = ""
	}

	return normalized.String()
}

// IsSameDomain проверяет что URL в пределах одного домена
func IsSameDomain(linkURL, baseURL *url.URL) bool {
	return linkURL.Host == baseURL.Host
}

// ResolveURL преобразует относительный URL в абсолютный
func ResolveURL(href string, baseURL *url.URL) string {
	if href == "" {
		return ""
	}

	if strings.HasPrefix(href, "#") ||
		strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "tel:") {
		return ""
	}

	parsedURL, err := url.Parse(href)
	if err != nil {
		return ""
	}

	if parsedURL.Scheme != "" && parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ""
	}

	resolvedURL := baseURL.ResolveReference(parsedURL)
	return NormalizeURL(resolvedURL)
}
