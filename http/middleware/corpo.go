package middleware

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
)

// CorpoMax sono i byte massimi del corpo di una richiesta. Il corpo vero
// piu' grande e' un blocco della trascrizione del terminale: 64 KiB in
// base64, circa 88 KB di JSON. Le rotte del pannello mandano elenchi di id
// e form, molto sotto.
const CorpoMax = 1 << 20

// LimiteCorpo limita il corpo di ogni richiesta a limite byte. Con
// Content-Length oltre limite risponde 413 senza leggere il corpo, con
// {"error": ...} e sotto /api/admin/ nella forma del pannello; senza
// Content-Length il corpo si legge al massimo fino a limite, e oltre la
// lettura fallisce, quindi il bind della rotta risponde come a un corpo
// sbagliato. In tutti e due i casi un warn con metodo, rotta e request_id,
// senza il corpo.
func LimiteCorpo(limite int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limite {
			corpoOltre(c, limite)
			msg := response.TranslateMsg(c, "BodyTooLarge")
			// Il pannello legge {code, message, data}, il client {error}.
			var corpo any = response.ErrorResponse{Error: msg}
			if strings.HasPrefix(c.Request.URL.Path, "/api/admin/") {
				corpo = response.Response{Code: 101, Message: msg}
			}
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, corpo)
			return
		}
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			c.Request.Body = &corpoLimitato{ReadCloser: http.MaxBytesReader(c.Writer, c.Request.Body, limite), c: c, limite: limite}
		}
		c.Next()
	}
}

// corpoLimitato scrive il warn la prima volta che la lettura passa il
// limite.
type corpoLimitato struct {
	io.ReadCloser
	c       *gin.Context
	limite  int64
	scritto bool
}

func (l *corpoLimitato) Read(p []byte) (int, error) {
	n, err := l.ReadCloser.Read(p)
	var oltre *http.MaxBytesError
	if !l.scritto && errors.As(err, &oltre) {
		l.scritto = true
		corpoOltre(l.c, l.limite)
	}
	return n, err
}

func corpoOltre(c *gin.Context, limite int64) {
	global.Logger.Per(c.Request.Context()).Warnf("%s %s: corpo della richiesta oltre %d byte, scartato", c.Request.Method, c.FullPath(), limite)
}
