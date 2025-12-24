package crawler

import (
	"context"
	"time"
)

// RateLimiter ограничивает скорость запросов
type RateLimiter struct {
	ticker <-chan time.Time
}

// NewRateLimiter создает новый rate limiter с заданной задержкой
func NewRateLimiter(ctx context.Context, delay time.Duration) *RateLimiter {
	// Если нет задержки, возвращаем rate limiter без тикера
	if delay <= 0 {
		return &RateLimiter{ticker: nil}
	}

	// Создаем канал с тикером
	t := time.NewTicker(delay)
	outChan := make(chan time.Time, 1)

	// В goroutine отправляем тики в канал пока контекст не отменен
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

// Wait ждет разрешения на отправку запроса
func (rl *RateLimiter) Wait(ctx context.Context) bool {
	if rl == nil || rl.ticker == nil {
		// Rate limiting отключен
		return true
	}

	select {
	case <-rl.ticker:
		return true
	case <-ctx.Done():
		return false
	}
}
