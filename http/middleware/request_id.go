package middleware

import (
	"crypto/rand"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// RequestId da' a ogni richiesta un id casuale, che mette nell'header di
// risposta X-Request-Id e nel contesto della richiesta: le righe scritte con
// global.Logger.Per(c.Request.Context()) lo hanno come request_id. Un
// X-Request-Id in arrivo non si usa: l'id lo sceglie l'API, non il chiamante.
// A fine richiesta scrive a debug metodo, rotta, stato e durata; l'URL con la
// query e l'IP no.
func RequestId() gin.HandlerFunc {
	return func(c *gin.Context) {
		inizio := time.Now()
		id := rand.Text()
		c.Header("X-Request-Id", id)
		ctx := logger.ConId(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		global.Logger.Per(ctx).Slog().Debug("richiesta",
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(inizio).Milliseconds())
	}
}
