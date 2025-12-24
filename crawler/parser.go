package crawler

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// HTMLParser отвечает за парсинг HTML
type HTMLParser struct{}

// NewHTMLParser создает новый парсер
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// ExtractLinks извлекает все ссылки из HTML
func (p *HTMLParser) ExtractLinks(htmlContent string, pageURL *url.URL) []string {
	links := []string{}
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					if link := ResolveURL(attr.Val, pageURL); link != "" {
						links = append(links, link)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return links
}
