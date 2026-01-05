package parser

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"code/internal/urlutil"
)

type AssetInfo struct {
	URL       string
	AssetType string
}

// HTMLParser парсит HTML и извлекает ссылки и ассеты
type HTMLParser struct{}

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
				if src := getAttr(n, "src"); src != "" {
					if resolved := urlutil.ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, AssetInfo{URL: resolved, AssetType: "image"})
					}
				}
			case "script":
				if src := getAttr(n, "src"); src != "" {
					if resolved := urlutil.ResolveURL(src, pageURL); resolved != "" {
						assets = append(assets, AssetInfo{URL: resolved, AssetType: "script"})
					}
				}
			case "link":
				if rel := getAttr(n, "rel"); rel == "stylesheet" {
					if href := getAttr(n, "href"); href != "" {
						if resolved := urlutil.ResolveURL(href, pageURL); resolved != "" {
							assets = append(assets, AssetInfo{URL: resolved, AssetType: "style"})
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

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
