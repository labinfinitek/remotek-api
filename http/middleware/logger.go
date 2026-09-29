package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
)

// Logger scrive a debug una riga per richiesta.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		global.Logger.Slog().Debug("Request",
			"uri", c.Request.URL.String(),
			"ip", c.ClientIP(),
			"method", c.Request.Method)
		c.Next()
	}
}
