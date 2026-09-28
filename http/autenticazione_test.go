package http

// Test delle letture del modulo auth sul router vero: un errore del database
// non fa uscire il tecnico dal client o dal pannello.

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestRustAuthLetturaFallita prova sul router vero RustAuth, il middleware
// delle rotte autenticate del client, quando il database non legge il token
// o il suo utente: la risposta e' 500 {"error": "Errore di sistema."}, con
// l'errore nel log. Prima era 401, che per il client 1.4.9 e' un logout
// forzato, come su /api/currentUser il 400 (user_model.dart:81-82). Un token
// che non c'e', scaduto, di un utente disabilitato o che non c'e' piu' ha il
// 401 di prima (golden currentUser-dopo-logout).
func TestRustAuthLetturaFallita(t *testing.T) {
	const erroreDiSistema, nonAutorizzato = `{"error":"Errore di sistema."}`, `{"error":"Unauthorized"}`
	scadenza := time.Now().Add(time.Hour).Unix()
	for _, tc := range []struct {
		nome, metodo, rotta, token string
		prepara                    func(t *testing.T, utente *model.User)
		stato                      int
		risposta                   string
	}{
		{"token non letto", "POST", "/api/currentUser", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "user_tokens", "") }, 500, erroreDiSistema},
		{"utente non letto", "POST", "/api/currentUser", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "users", "") }, 500, erroreDiSistema},
		{"utente non letto", "GET", "/api/users", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "users", "") }, 500, erroreDiSistema},
		{"token che non c'e'", "POST", "/api/currentUser", "token-che-non-esiste",
			func(*testing.T, *model.User) {}, 401, nonAutorizzato},
		{"token scaduto", "POST", "/api/currentUser", "token-scaduto", func(t *testing.T, utente *model.User) {
			crea(t, &model.UserToken{UserId: utente.Id, Token: "token-scaduto", ExpiredAt: time.Now().Add(-time.Hour).Unix()})
		}, 401, nonAutorizzato},
		{"utente disabilitato", "POST", "/api/currentUser", tokenDelPannello, func(t *testing.T, utente *model.User) {
			if err := service.DB.Model(utente).Update("status", model.COMMON_STATUS_DISABLED).Error; err != nil {
				t.Fatal(err)
			}
		}, 401, nonAutorizzato},
		{"utente che non c'e' piu'", "POST", "/api/currentUser", "token-senza-utente", func(t *testing.T, _ *model.User) {
			crea(t, &model.UserToken{UserId: 999999, Token: "token-senza-utente", ExpiredAt: scadenza})
		}, 401, nonAutorizzato},
	} {
		t.Run(tc.nome+" "+tc.rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			precJwt := global.Jwt
			global.Jwt = jwt.NewJwt("", time.Hour) // senza chiave: RustAuth cerca il token nel database
			t.Cleanup(func() { global.Jwt = precJwt })
			tc.prepara(t, utente)

			req := httptest.NewRequest(tc.metodo, tc.rotta, strings.NewReader(`{"id":"999000111","uuid":"dXVpZA=="}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)
			if got := rec.Body.String(); rec.Code != tc.stato || got != tc.risposta {
				t.Errorf("%s %s: %d %s, attesi %d e %s", tc.metodo, tc.rotta, rec.Code, got, tc.stato, tc.risposta)
			}
			if nelLog := registro.String(); tc.stato == 500 && (!strings.Contains(nelLog, tc.metodo+" "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestBackendUserAuthLetturaFallita prova sul router vero BackendUserAuth, il
// middleware del pannello, e la configurazione del pannello, che legge il
// token da sola, quando il database non legge il token o il suo utente: la
// risposta e' code 101 "Errore di sistema.", con l'errore nel log. Prima il
// middleware rispondeva code 403 NeedLogin, che per il pannello e' un logout
// (request.js), e la configurazione rispondeva come senza login. Un token che
// non c'e' ha le risposte di prima.
func TestBackendUserAuthLetturaFallita(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	for _, tc := range []struct {
		rotta, token, tabella, risposta string
	}{
		{"/api/admin/user/current", tokenDelPannello, "user_tokens", erroreDiSistema},
		{"/api/admin/user/current", tokenDelPannello, "users", erroreDiSistema},
		{"/api/admin/config/admin", tokenDelPannello, "user_tokens", erroreDiSistema},
		{"/api/admin/config/admin", tokenDelPannello, "users", erroreDiSistema},
		{"/api/admin/user/current", "token-che-non-esiste", "", `{"code":403,"message":"NEEDLOGIN","data":null}`},
		{"/api/admin/config/admin", "token-che-non-esiste", "", `{"code":0,"message":"success","data":{"title":"TITOLO"}}`},
	} {
		t.Run(tc.rotta+" "+tc.token+" "+tc.tabella, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			if tc.tabella != "" {
				rifiutaLetture(t, tc.tabella, "")
			}
			rec := conToken(g, "GET", tc.rotta, tc.token)
			accedi, err := global.Localizer("").LocalizeMessage(&i18n.Message{ID: "NeedLogin"})
			if err != nil {
				t.Fatal(err)
			}
			risposta := strings.NewReplacer("TITOLO", global.Config.Admin.Title, "NEEDLOGIN", accedi).Replace(tc.risposta)
			if got := rec.Body.String(); rec.Code != 200 || got != risposta {
				t.Errorf("GET %s: stato %d\n got  %s\n want %s", tc.rotta, rec.Code, got, risposta)
			}
			if nelLog := registro.String(); tc.tabella != "" && (!strings.Contains(nelLog, "GET "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("GET %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}
		})
	}
}
