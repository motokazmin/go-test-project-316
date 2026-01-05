package state

import (
	"net/url"
	"sync"

	"code/internal/httputil"
)

// URLWithDepth представляет URL с его глубиной в дереве обхода
type URLWithDepth struct {
	URL   string
	Depth int
}

// URLQueue — потокобезопасная очередь URL для обхода
type URLQueue struct {
	items []URLWithDepth
	mu    sync.Mutex
}

func NewURLQueue(rootURL string) *URLQueue {
	return &URLQueue{
		items: []URLWithDepth{{URL: rootURL, Depth: 0}},
	}
}

func (q *URLQueue) Enqueue(urls []URLWithDepth) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, urls...)
}

func (q *URLQueue) Dequeue() *URLWithDepth {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	item := q.items[0]
	q.items = q.items[1:]
	return &item
}

func (q *URLQueue) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) == 0
}

// VisitedSet — потокобезопасное множество посещённых URL
type VisitedSet struct {
	urls map[string]bool
	mu   sync.Mutex
}

func NewVisitedSet() *VisitedSet {
	return &VisitedSet{
		urls: make(map[string]bool),
	}
}

func (v *VisitedSet) Add(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.urls[url] = true
}

func (v *VisitedSet) Contains(url string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.urls[url]
}

type CrawlState struct {
	BaseURL     *url.URL
	Queue       *URLQueue
	Visited     *VisitedSet
	Semaphore   chan struct{}
	WG          sync.WaitGroup
	RateLimiter *httputil.RateLimiter
}

func NewCrawlState(rootURL *url.URL, workers int, rateLimiter *httputil.RateLimiter) *CrawlState {
	return &CrawlState{
		BaseURL:     rootURL,
		Queue:       NewURLQueue(rootURL.String()),
		Visited:     NewVisitedSet(),
		Semaphore:   make(chan struct{}, workers),
		RateLimiter: rateLimiter,
	}
}
