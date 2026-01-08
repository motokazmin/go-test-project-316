package urlutil

import (
	"net/url"
	"testing"
)

func TestNormalizeURLString(t *testing.T) {
	if got := NormalizeURLString("example.com"); got != "https://example.com" {
		t.Fatalf("expected https://example.com, got %s", got)
	}

	if got := NormalizeURLString("http://example.com"); got != "http://example.com" {
		t.Fatalf("expected http://example.com, got %s", got)
	}
}

func TestParseAndValidateURL(t *testing.T) {
	u, err := ParseAndValidateURL("example.com/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Scheme != "https" || u.Host != "example.com" {
		t.Fatalf("unexpected parsed url: %s", u.String())
	}

	if _, err := ParseAndValidateURL("://missing"); err == nil {
		t.Fatalf("expected error for invalid url")
	}

	if _, err := ParseAndValidateURL("https://"); err == nil {
		t.Fatalf("expected error for missing host")
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/root")

	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{"relative path", "/page", "https://example.com/page"},
		{"fragment skipped", "#anchor", ""},
		{"javascript skipped", "javascript:void(0)", ""},
		{"mailto skipped", "mailto:test@example.com", ""},
		{"invalid scheme", "ftp://example.com/file", ""},
		{"empty href", "", ""},
	}

	for _, tt := range tests {
		if got := ResolveURL(tt.href, base); got != tt.expected {
			t.Fatalf("%s: expected %s, got %s", tt.name, tt.expected, got)
		}
	}

	abs := ResolveURL("https://example.com/page#frag", base)
	if abs != "https://example.com/page" {
		t.Fatalf("expected fragment to be stripped, got %s", abs)
	}
}
