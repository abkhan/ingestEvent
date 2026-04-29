package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func RateLimit(rps int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), rps)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too Many Requests"})
			return
		}
		c.Next()
	}
}
