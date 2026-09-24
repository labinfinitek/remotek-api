package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestTrustProxyVuoto verifica che con gin.trust-proxy vuoto, il default,
// X-Forwarded-For e X-Real-IP non cambino ClientIP(), l'IP che captcha e ban
// usano. Il caso del proxy fidato prova che le stesse intestazioni, da un
// proxy fidato, lo cambierebbero: senza, il primo caso passerebbe anche se
// gin non le leggesse mai.
func TestTrustProxyVuoto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const remoto = "192.0.2.10"
	for _, tc := range []struct{ name, trustProxy, want string }{
		{"vuoto: nessun proxy fidato", "", remoto},
		{"richiesta da un proxy fidato", remoto, "203.0.113.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := gin.New()
			if err := setTrustedProxies(g, tc.trustProxy); err != nil {
				t.Fatalf("setTrustedProxies(%q): %v", tc.trustProxy, err)
			}
			g.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
			req.RemoteAddr = remoto + ":40000"
			req.Header.Set("X-Forwarded-For", "203.0.113.7")
			req.Header.Set("X-Real-IP", "203.0.113.8")
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)
			if got := rec.Body.String(); got != tc.want {
				t.Errorf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
