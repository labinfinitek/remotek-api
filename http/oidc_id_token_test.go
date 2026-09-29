package http

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestIdTokenRichiesto prova sul router vero che il login OIDC si ferma, con
// OauthFailed e senza creare utenti ne' associazioni, se l'id_token manca,
// ha un nonce che non e' quello del login, un sub che non e' quello della
// userinfo o nessun sub (OIDC Core 2, 3.1.3.3 e 5.3.2); il motivo va nel log.
// Prima senza id_token il login andava avanti con la sola userinfo, un nonce
// nell'id_token di un login senza nonce passava, il sub della userinfo non si
// confrontava, e senza sub ne' nell'id_token ne' nella userinfo l'associazione
// nasceva con open_id vuoto.
func TestIdTokenRichiesto(t *testing.T) {
	for _, tc := range []struct {
		nome    string
		idToken idTokenFinto
		inCache bool // il login e' nella cache senza nonce, invece che da /api/oidc/auth
		nelLog  string
	}{
		{"senza id_token", idTokenFinto{assente: true}, false, "il provider non ha mandato l'id_token: controllare che gli scope comprendano openid"},
		{"nonce diverso", idTokenFinto{nonce: "nonce-di-un-altro-login"}, false, "Nonce does not match"},
		{"nonce nell'id_token di un login senza nonce", idTokenFinto{nonce: "nonce-di-un-altro-login"}, true, "Nonce does not match"},
		{"sub diverso dalla userinfo", idTokenFinto{sub: "sub-2"}, false, "il sub della userinfo non e' quello dell'id_token"},
		{"senza sub", idTokenFinto{senzaSub: true}, false, "l'id_token non ha il sub"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			registra := true
			crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
				Issuer: providerOidcCon(t, tc.idToken), AutoRegister: &registra})
			codice := "prova-callback"
			if tc.inCache {
				service.AllService.OauthService.SetOauthCache(codice, &service.OauthCacheItem{Op: "aziendale", Action: service.OauthActionTypeLogin}, 0)
			} else {
				rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
				var risposta struct{ Code string }
				if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 {
					t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
				}
				codice = risposta.Code
			}
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(codice) })
			utenti, associazioni := righe(t, "users"), righe(t, "user_thirds")

			rec := richiesta(g, "GET", "/api/oidc/callback?state="+codice+"&code="+codiceDelProvider, "", "")
			if !strings.Contains(rec.Body.String(), "var msg = 'OauthFailed'") {
				t.Errorf("GET /api/oidc/callback: %d, atteso il messaggio OauthFailed\n%s", rec.Code, rec.Body)
			}
			if n, m := righe(t, "users"), righe(t, "user_thirds"); n != utenti || m != associazioni {
				t.Errorf("dopo il callback %d utenti e %d associazioni, erano %d e %d", n, m, utenti, associazioni)
			}
			if voce := service.AllService.OauthService.GetOauthCache(codice); voce == nil || voce.UserId != 0 || voce.OpenId != "" {
				t.Errorf("login in cache dopo il callback: %+v, atteso senza utente ne' account", voce)
			}
			if !strings.Contains(registro.String(), tc.nelLog) {
				t.Errorf("nel log manca %q:\n%s", tc.nelLog, registro)
			}
		})
	}
}

// TestScopeSenzaOpenid prova sul router vero che un provider salvato con gli
// scope "profile,email", senza openid, chiede lo stesso openid al provider e
// il login riesce: prima l'indirizzo di autorizzazione non aveva openid, e un
// provider vero non avrebbe mandato l'id_token.
func TestScopeSenzaOpenid(t *testing.T) {
	g, _, _ := pannello(t, false)
	registra := true
	crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
		Issuer: providerOidc(t), AutoRegister: &registra, Scopes: "profile,email"})

	rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
	var risposta struct{ Code, Url string }
	if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 {
		t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
	}
	t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(risposta.Code) })
	indirizzo, err := url.Parse(risposta.Url)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := indirizzo.Query().Get("scope"), "openid profile email"; got != want {
		t.Errorf("POST /api/oidc/auth: scope %q nell'indirizzo del provider, attesi %q", got, want)
	}

	rec = richiesta(g, "GET", "/api/oidc/callback?state="+risposta.Code+"&code="+codiceDelProvider, "", "")
	if !strings.Contains(rec.Body.String(), "var msg = 'OauthSuccess'") {
		t.Errorf("GET /api/oidc/callback: %d %s", rec.Code, rec.Body)
	}
}
