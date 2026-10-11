package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// IntegrazioneAuth lascia passare solo le richieste con
// "Authorization: Bearer <token>" di un token di integrazione valido
// (ADR-0026): un token di login non vale. Senza token, o con un token
// sconosciuto, scaduto o revocato, 401 {"error": "Unauthorized"}; un errore
// del database 500. Il token non va mai nel log; il nome di un token
// scaduto o revocato si', a livello warn.
func IntegrazioneAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		t, err := service.AllService.TokenIntegrazioneService.Verifica(token, time.Now())
		switch {
		case errors.Is(err, service.ErrNotFound):
			if t != nil {
				global.Logger.Per(c.Request.Context()).Warnf("%s %s: token di integrazione %s scaduto o revocato", c.Request.Method, c.FullPath(), t.Nome)
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		case err != nil:
			response.ErrorStatusErr(c, http.StatusInternalServerError, "SystemError", err)
			c.Abort()
			return
		}
		c.Next()
	}
}
