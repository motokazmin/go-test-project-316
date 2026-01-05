package crawler

import (
	"net/http"
	"time"

	"code/internal/checker"
	"code/internal/httputil"
	"code/internal/report"
	"code/internal/seo"
)

type HTTPClient = httputil.HTTPClient

type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  bool
	HTTPClient  HTTPClient
}

type (
	Report     = report.Report
	Page       = report.Page
	BrokenLink = checker.BrokenLink
	SEO        = seo.SEO
	Asset      = checker.Asset
)

func normalizeOptions(opts *Options) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
}
