package crawler

import (
	"net/url"
	"sync"
)

// URLQueue управляет очередью URL для обхода
type URLQueue struct {
	items []urlWithDepth
	mu    sync.Mutex
}

// NewURLQueue создает новую очередь
func NewURLQueue(rootURL string) *URLQueue {
	return &URLQueue{
		items: []urlWithDepth{{url: rootURL, depth: 0}},
	}
}

// Enqueue добавляет URL в очередь
func (q *URLQueue) Enqueue(urls []urlWithDepth) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, urls...)
}

// Dequeue извлекает следующий URL
func (q *URLQueue) Dequeue() *urlWithDepth {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	item := q.items[0]
	q.items = q.items[1:]
	return &item
}

// IsEmpty проверяет пуста ли очередь
func (q *URLQueue) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) == 0
}

// VisitedSet управляет множеством посещенных URL
type VisitedSet struct {
	urls map[string]bool
	mu   sync.Mutex
}

// NewVisitedSet создает новое множество
func NewVisitedSet() *VisitedSet {
	return &VisitedSet{
		urls: make(map[string]bool),
	}
}

// Add добавляет URL в множество
func (v *VisitedSet) Add(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.urls[url] = true
}

// Contains проверяет наличие URL
func (v *VisitedSet) Contains(url string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.urls[url]
}

// CrawlState содержит состояние процесса краулирования
type CrawlState struct {
	BaseURL     *url.URL
	Queue       *URLQueue
	Visited     *VisitedSet
	Semaphore   chan struct{}
	WG          sync.WaitGroup
	RateLimiter *RateLimiter
}

// NewCrawlState создает новое состояние
func NewCrawlState(rootURL *url.URL, workers int, rateLimiter *RateLimiter) *CrawlState {
	return &CrawlState{
		BaseURL:     rootURL,
		Queue:       NewURLQueue(rootURL.String()),
		Visited:     NewVisitedSet(),
		Semaphore:   make(chan struct{}, workers),
		RateLimiter: rateLimiter,
	}
}
