package seo

import (
	"strings"

	"golang.org/x/net/html"
)

// SEO содержит базовые SEO параметры страницы
type SEO struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
}

// Extractor извлекает SEO данные из HTML
type Extractor struct{}

// NewExtractor создает новый экстрактор
func NewExtractor() *Extractor {
	return &Extractor{}
}

// Extract извлекает SEO параметры
func (e *Extractor) Extract(htmlContent string) *SEO {
	seo := &SEO{
		HasTitle:       false,
		HasDescription: false,
		HasH1:          false,
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return seo
	}

	e.extractTitle(doc, seo)
	e.extractDescription(doc, seo)
	e.extractH1(doc, seo)

	return seo
}

// extractTitle извлекает title
func (e *Extractor) extractTitle(doc *html.Node, seo *SEO) {
	var find func(*html.Node)
	find = func(n *html.Node) {
		// Если уже нашли title, не ищем дальше
		if seo.HasTitle {
			return
		}

		if n.Type == html.ElementNode && n.Data == "title" {
			seo.HasTitle = true
			// Используем extractTextContent для корректной работы с XML
			seo.Title = extractTextContent(n)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
}

// extractDescription извлекает meta description
func (e *Extractor) extractDescription(doc *html.Node, seo *SEO) {
	var find func(*html.Node)
	find = func(n *html.Node) {
		// Если уже нашли description, не ищем дальше
		if seo.HasDescription {
			return
		}

		if n.Type == html.ElementNode && n.Data == "meta" {
			name := ""
			content := ""

			for _, attr := range n.Attr {
				if attr.Key == "name" && attr.Val == "description" {
					name = "description"
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}

			if name == "description" {
				seo.HasDescription = true
				seo.Description = content
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
}

// extractH1 извлекает H1
func (e *Extractor) extractH1(doc *html.Node, seo *SEO) {
	var find func(*html.Node)
	find = func(n *html.Node) {
		// Если уже нашли H1, не ищем дальше
		if seo.HasH1 {
			return
		}

		if n.Type == html.ElementNode && n.Data == "h1" {
			seo.HasH1 = true
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
}

// extractTextContent извлекает текст из узла с нормализацией пробелов
func extractTextContent(n *html.Node) string {
	if n == nil {
		return ""
	}

	var parts []string
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			// Разбиваем на слова и убираем пустые
			words := strings.Fields(node.Data)
			parts = append(parts, words...)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(n)
	return strings.Join(parts, " ")
}
