package parser

import (
	"net/url"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	html := `
        <html>
        <body>
            <a href="/about">About</a>
            <a href="https://example.com/contact#fragment">Contact</a>
            <a href="#anchor">Anchor</a>
            <a href="javascript:void(0)">JS</a>
            <a href="mailto:test@example.com">Mail</a>
            <a href="ftp://example.com/file">FTP</a>
        </body>
        </html>
    `

	parser := NewHTMLParser()
	base, _ := url.Parse("https://example.com/root")

	links := parser.ExtractLinks(html, base)

	expected := map[string]bool{
		"https://example.com/about":   false,
		"https://example.com/contact": false,
	}

	for _, link := range links {
		if _, ok := expected[link]; ok {
			expected[link] = true
		} else {
			t.Fatalf("unexpected link: %s", link)
		}
	}

	for link, seen := range expected {
		if !seen {
			t.Fatalf("expected link %s to be extracted", link)
		}
	}
}
