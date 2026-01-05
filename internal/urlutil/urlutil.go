package urlutil

import (
	"fmt"
	"net/url"
	"strings"
)

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

func NormalizeURLString(urlStr string) string {
	if !strings.Contains(urlStr, "://") {
		return "https://" + urlStr
	}
	return urlStr
}

// NormalizeURL убирает fragment и trailing slash для избежания дубликатов
func NormalizeURL(u *url.URL) string {
	normalized := *u
	normalized.Fragment = ""

	if normalized.Path == "/" {
		normalized.Path = ""
	}

	return normalized.String()
}

func IsSameDomain(linkURL, baseURL *url.URL) bool {
	return linkURL.Host == baseURL.Host
}

// ResolveURL преобразует относительный URL в абсолютный.
// Пропускает: якоря, javascript:, mailto:, tel:
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
