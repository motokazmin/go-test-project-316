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
	if seo.Title != nil {
		t.Error("Title should be nil when no title element")
	}

	if seo.HasDescription {
		t.Error("HasDescription should be false when no meta description")
	}
	if seo.Description != nil {
		t.Error("Description should be nil when no meta description")
	}

	if seo.HasH1 {
		t.Error("HasH1 should be false when no h1 element")
	}
	if seo.H1 != nil {
		t.Error("H1 should be nil when no h1 element")
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
	if seo.Title == nil {
		t.Fatal("Title should not be nil when title element exists")
	}
	if *seo.Title != "" {
		t.Errorf("Title should be empty string, got: %s", *seo.Title)
	}

	// Description
	if !seo.HasDescription {
		t.Error("HasDescription should be true when meta description exists")
	}
	if seo.Description == nil {
		t.Fatal("Description should not be nil when meta description exists")
	}
	if *seo.Description != "" {
		t.Errorf("Description should be empty string, got: %s", *seo.Description)
	}

	// H1
	if !seo.HasH1 {
		t.Error("HasH1 should be true when h1 element exists")
	}
	if seo.H1 == nil {
		t.Fatal("H1 should not be nil when h1 element exists")
	}
	if *seo.H1 != "" {
		t.Errorf("H1 should be empty string, got: %s", *seo.H1)
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
	if seo.Title == nil {
		t.Fatal("Title should not be nil")
	}
	if *seo.Title != "My Page Title" {
		t.Errorf("Expected 'My Page Title', got: %s", *seo.Title)
	}

	// Description
	if !seo.HasDescription {
		t.Error("HasDescription should be true")
	}
	if seo.Description == nil {
		t.Fatal("Description should not be nil")
	}
	if *seo.Description != "This is my page description" {
		t.Errorf("Expected 'This is my page description', got: %s", *seo.Description)
	}

	// H1
	if !seo.HasH1 {
		t.Error("HasH1 should be true")
	}
	if seo.H1 == nil {
		t.Fatal("H1 should not be nil")
	}
	if *seo.H1 != "Main Heading" {
		t.Errorf("Expected 'Main Heading', got: %s", *seo.H1)
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
	if *seo.Title != "Spaced Title" {
		t.Errorf("Expected 'Spaced Title', got: '%s'", *seo.Title)
	}

	// H1 должен быть объединен без лишних пробелов
	if *seo.H1 != "Multi Line H1" {
		t.Errorf("Expected 'Multi Line H1', got: '%s'", *seo.H1)
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
	if *seo.Title != "First Title" {
		t.Errorf("Expected first title, got: %s", *seo.Title)
	}

	if *seo.H1 != "First H1" {
		t.Errorf("Expected first H1, got: %s", *seo.H1)
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

	// Должен извлечь весь текст
	expected := "Text with bold and italic"
	if *seo.H1 != expected {
		t.Errorf("Expected '%s', got: '%s'", expected, *seo.H1)
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
