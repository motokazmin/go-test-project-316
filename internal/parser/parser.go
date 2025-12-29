package parser

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"code/internal/urlutil"
)

// AssetInfo содержит информацию об ассете из HTML
type AssetInfo struct {
	URL       string
	AssetType string
}

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
					if link := urlutil.ResolveURL(attr.Val, pageURL); link != "" {
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

// ExtractAssets извлекает ассеты (изображения, скрипты, стили) из HTML
func (p *HTMLParser) ExtractAssets(htmlContent string, pageURL *url.URL) []AssetInfo {
	assets := []AssetInfo{}
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return assets
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "img":
				// <img src="...">
				if src := getAttr(n, "src"); src != "" {
					if resolved := urlutil.ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, AssetInfo{
							URL:       resolved,
							AssetType: "image",
						})
					}
				}
			case "script":
				// <script src="...">
				if src := getAttr(n, "src"); src != "" {
					if resolved := urlutil.ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, AssetInfo{
							URL:       resolved,
							AssetType: "script",
						})
					}
				}
			case "link":
				// <link rel="stylesheet" href="...">
				if rel := getAttr(n, "rel"); rel == "stylesheet" {
					if href := getAttr(n, "href"); href != "" {
						if resolved := urlutil.ResolveURL(href, pageURL); resolved != "" {
							assets = append(assets, AssetInfo{
								URL:       resolved,
								AssetType: "style",
							})
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return assets
}

// getAttr получает значение атрибута узла
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
