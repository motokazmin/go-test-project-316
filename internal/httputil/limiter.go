package httputil

import (
	"context"
	"time"
)

// RateLimiter ограничивает скорость запросов
type RateLimiter struct {
	ticker <-chan time.Time
}

func NewRateLimiter(ctx context.Context, delay time.Duration) *RateLimiter {
	if delay <= 0 {
		return &RateLimiter{ticker: nil}
	}

	t := time.NewTicker(delay)
	outChan := make(chan time.Time, 1)

	go func() {
		defer t.Stop()
		defer close(outChan)

		for {
			select {
			case <-ctx.Done():
				return
			case tick := <-t.C:
				select {
				case outChan <- tick:
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()

	return &RateLimiter{ticker: outChan}
}

// Wait ждёт разрешения на отправку запроса
func (rl *RateLimiter) Wait(ctx context.Context) bool {
	if rl == nil || rl.ticker == nil {
		return true
	}

	select {
	case <-rl.ticker:
		return true
	case <-ctx.Done():
		return false
	}
}
