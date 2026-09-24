package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
)

// TestPasswordNuovaCortaRifiutata manda una password di 14 caratteri ai tre
// percorsi dove una password si crea o si cambia: registrazione, password
// impostata dal pannello (changePwd) e cambio della propria (changeCurPwd).
// Ognuno deve rispondere come agli altri errori dei form: 200, code 101 e il
// messaggio del validatore. Col codice di prima la password passa il
// validatore e la richiesta arriva ai servizi, che qui non ci sono: 500.
func TestPasswordNuovaCortaRifiutata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	global.ApiInitValidator()
	prec := global.Config.App.Register
	global.Config.App.Register = true
	t.Cleanup(func() { global.Config.App.Register = prec })

	ct := &User{}
	g := gin.New()
	g.Use(gin.RecoveryWithWriter(io.Discard))
	g.POST("/register", ct.Register)
	g.POST("/changePwd", ct.UpdatePassword)
	g.POST("/changeCurPwd", ct.ChangeCurPwd)

	corta := strings.Repeat("a", 14)
	for _, tc := range []struct {
		percorso string
		corpo    map[string]any
		campo    string
	}{
		{"/register", map[string]any{"username": "collaudo", "password": corta, "confirm_password": corta}, "Password"},
		{"/changePwd", map[string]any{"id": 2, "password": corta}, "Password"},
		{"/changeCurPwd", map[string]any{"old_password": "abcd", "new_password": corta}, "NewPassword"},
	} {
		t.Run(tc.percorso[1:], func(t *testing.T) {
			dati, err := json.Marshal(tc.corpo)
			if err != nil {
				t.Fatalf("corpo: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, tc.percorso, bytes.NewReader(dati))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept-Language", "en")
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)

			want := `{"code":101,"message":"` + tc.campo + ` must be at least 15 characters in length","data":null}`
			if rec.Code != http.StatusOK || rec.Body.String() != want {
				t.Errorf("stato %d, corpo %s; atteso 200, %s", rec.Code, rec.Body.String(), want)
			}
		})
	}
}
