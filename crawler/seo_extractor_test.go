package crawler

import (
	"testing"
)

func TestSEOExtractor_NoElements(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `<html><body><p>No SEO elements</p></body></html>`

	seo := extractor.Extract(html)

	if seo.HasTitle {
		t.Error("HasTitle should be false when no title element")
	}
	if seo.Title != "" {
		t.Error("Title should be empty string when no title element")
	}

	if seo.HasDescription {
		t.Error("HasDescription should be false when no meta description")
	}
	if seo.Description != "" {
		t.Error("Description should be empty string when no meta description")
	}

	if seo.HasH1 {
		t.Error("HasH1 should be false when no h1 element")
	}
}

func TestSEOExtractor_EmptyElements(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `
        <html>
        <head>
            <title></title>
            <meta name="description" content="">
        </head>
        <body>
            <h1></h1>
        </body>
        </html>
    `

	seo := extractor.Extract(html)

	// Title
	if !seo.HasTitle {
		t.Error("HasTitle should be true when title element exists")
	}
	if seo.Title != "" {
		t.Errorf("Title should be empty string, got: %s", seo.Title)
	}

	// Description
	if !seo.HasDescription {
		t.Error("HasDescription should be true when meta description exists")
	}
	if seo.Description != "" {
		t.Errorf("Description should be empty string, got: %s", seo.Description)
	}

	// H1
	if !seo.HasH1 {
		t.Error("HasH1 should be true when h1 element exists")
	}
}

func TestSEOExtractor_WithContent(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `
        <html>
        <head>
            <title>My Page Title</title>
            <meta name="description" content="This is my page description">
        </head>
        <body>
            <h1>Main Heading</h1>
        </body>
        </html>
    `

	seo := extractor.Extract(html)

	// Title
	if !seo.HasTitle {
		t.Error("HasTitle should be true")
	}
	if seo.Title != "My Page Title" {
		t.Errorf("Expected 'My Page Title', got: %s", seo.Title)
	}

	// Description
	if !seo.HasDescription {
		t.Error("HasDescription should be true")
	}
	if seo.Description != "This is my page description" {
		t.Errorf("Expected 'This is my page description', got: %s", seo.Description)
	}

	// H1
	if !seo.HasH1 {
		t.Error("HasH1 should be true")
	}
}

func TestSEOExtractor_WhitespaceHandling(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `
        <html>
        <head>
            <title>  Spaced Title  </title>
        </head>
        <body>
            <h1>
                Multi
                Line
                H1
            </h1>
        </body>
        </html>
    `

	seo := extractor.Extract(html)

	// Title должен быть без пробелов по краям
	if seo.Title != "Spaced Title" {
		t.Errorf("Expected 'Spaced Title', got: '%s'", seo.Title)
	}

	// H1 флаг должен быть установлен
	if !seo.HasH1 {
		t.Error("HasH1 should be true for multiline h1")
	}
}

func TestSEOExtractor_MultipleElements(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `
        <html>
        <head>
            <title>First Title</title>
            <title>Second Title</title>
        </head>
        <body>
            <h1>First H1</h1>
            <h1>Second H1</h1>
        </body>
        </html>
    `

	seo := extractor.Extract(html)

	// Должен быть взят первый элемент
	if seo.Title != "First Title" {
		t.Errorf("Expected first title, got: %s", seo.Title)
	}

	// H1 флаг должен быть установлен
	if !seo.HasH1 {
		t.Error("HasH1 should be true")
	}
}

func TestSEOExtractor_NestedText(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `
        <html>
        <body>
            <h1>Text with <strong>bold</strong> and <em>italic</em></h1>
        </body>
        </html>
    `

	seo := extractor.Extract(html)

	// H1 флаг должен быть установлен
	if !seo.HasH1 {
		t.Error("HasH1 should be true for nested content")
	}
}

func TestSEOExtractor_InvalidHTML(t *testing.T) {
	extractor := NewSEOExtractor()
	html := `<html><head><title>Valid Title`

	seo := extractor.Extract(html)

	// Даже с невалидным HTML парсер должен извлечь что может
	if !seo.HasTitle {
		t.Error("Should handle invalid HTML gracefully")
	}
}
