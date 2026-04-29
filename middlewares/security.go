package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}
