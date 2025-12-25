package crawler

import (
	"strings"

	"golang.org/x/net/html"
)

// SEOExtractor извлекает SEO данные из HTML
type SEOExtractor struct{}

// NewSEOExtractor создает новый экстрактор
func NewSEOExtractor() *SEOExtractor {
	return &SEOExtractor{}
}

// Extract извлекает SEO параметры
func (e *SEOExtractor) Extract(htmlContent string) *SEO {
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
func (e *SEOExtractor) extractTitle(doc *html.Node, seo *SEO) {
	var find func(*html.Node)
	find = func(n *html.Node) {
		// Если уже нашли title, не ищем дальше
		if seo.HasTitle {
			return
		}

		if n.Type == html.ElementNode && n.Data == "title" {
			seo.HasTitle = true

			// Извлекаем текст если есть
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				seo.Title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
}

// extractDescription извлекает meta description
func (e *SEOExtractor) extractDescription(doc *html.Node, seo *SEO) {
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
func (e *SEOExtractor) extractH1(doc *html.Node, seo *SEO) {
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
