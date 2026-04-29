package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type CircuitBreaker struct {
	failureThreshold int
	timeout          time.Duration
	failures         int
	lastFailTime     time.Time
	state            string // "closed", "open", "half-open"
	mu               sync.Mutex
}

func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		state:            "closed",
	}
}

func (cb *CircuitBreaker) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cb.mu.Lock()
		state := cb.state
		cb.mu.Unlock()

		if state == "open" {
			if time.Since(cb.lastFailTime) > cb.timeout {
				cb.mu.Lock()
				cb.state = "half-open"
				cb.mu.Unlock()
			} else {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service Unavailable"})
				return
			}
		}

		c.Next()

		cb.mu.Lock()
		if c.Writer.Status() >= 500 {
			cb.failures++
			if cb.failures >= cb.failureThreshold {
				cb.state = "open"
				cb.lastFailTime = time.Now()
			}
		} else if cb.state == "half-open" {
			cb.state = "closed"
			cb.failures = 0
		}
		cb.mu.Unlock()
	}
}
