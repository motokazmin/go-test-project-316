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

// ExtractAssets извлекает ассеты (изображения, скрипты, стили) из HTML
func (p *HTMLParser) ExtractAssets(htmlContent string, pageURL *url.URL) []assetInfo {
	assets := []assetInfo{}
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
					if resolved := ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, assetInfo{
							url:       resolved,
							assetType: "image",
						})
					}
				}
			case "script":
				// <script src="...">
				if src := getAttr(n, "src"); src != "" {
					if resolved := ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, assetInfo{
							url:       resolved,
							assetType: "script",
						})
					}
				}
			case "link":
				// <link rel="stylesheet" href="...">
				if rel := getAttr(n, "rel"); rel == "stylesheet" {
					if href := getAttr(n, "href"); href != "" {
						if resolved := ResolveURL(href, pageURL); resolved != "" {
							assets = append(assets, assetInfo{
								url:       resolved,
								assetType: "style",
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
