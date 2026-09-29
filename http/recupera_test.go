package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// TestPanicNelLog prova sul router vero che un panic in un gestore risponda
// 500 senza corpo, come gin.Recovery, e vada nel log in una riga error con
// metodo e rotta, senza gli header della richiesta.
func TestPanicNelLog(t *testing.T) {
	g, _, registro := pannello(t, false)
	g.GET("/prova/panic", func(*gin.Context) { panic("rotto dal test") })
	req := httptest.NewRequest("GET", "/prova/panic", nil)
	req.Header.Set("Cookie", "sessione=biscotto-segreto")
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != 500 || rec.Body.Len() != 0 {
		t.Errorf("GET /prova/panic: %d %q, attesi 500 e nessun corpo", rec.Code, rec.Body)
	}
	nelLog := registro.String()
	if logger.Conta(nelLog, "ERROR", "GET /prova/panic: panic: rotto dal test") != 1 {
		t.Errorf("nel log manca la riga error del panic:\n%s", nelLog)
	}
	if strings.Contains(nelLog, "biscotto-segreto") {
		t.Errorf("il cookie della richiesta e' nel log:\n%s", nelLog)
	}
}
