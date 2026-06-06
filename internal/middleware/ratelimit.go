package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// LoginRateLimit limits login requests per IP within a time window.
func LoginRateLimit(maxRequests int, window time.Duration) fiber.Handler {
	type entry struct {
		count   int
		resetAt time.Time
	}

	var mu sync.Mutex
	ipCounts := make(map[string]*entry)

	// Periodic cleanup of expired entries
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, e := range ipCounts {
				if now.After(e.resetAt) {
					delete(ipCounts, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c fiber.Ctx) error {
		ip := c.IP()

		mu.Lock()
		e, exists := ipCounts[ip]
		now := time.Now()

		if !exists || now.After(e.resetAt) {
			ipCounts[ip] = &entry{count: 1, resetAt: now.Add(window)}
			mu.Unlock()
			return c.Next()
		}

		e.count++
		if e.count > maxRequests {
			mu.Unlock()
			return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, try again later",
			})
		}
		mu.Unlock()

		return c.Next()
	}
}
