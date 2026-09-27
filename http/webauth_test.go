package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// scaduto e' la risposta del pannello a un codice che non c'e' o che la
// rotta non accetta (OauthExpired, lingua di conf/config.yaml).
const scaduto = `{"code":101,"message":"Autorizzazione OAuth scaduta, riprova.","data":null}`

// TestWebauthSpento prova sul router vero che con app.web-sso spento, il
// default di conf/config.yaml, webauth non si avvia: /api/oidc/auth del
// client risponde come a un op che non esiste (golden
// oidc-auth-op-sconosciuto), e cosi' l'associazione dal pannello.
func TestWebauthSpento(t *testing.T) {
	g, _, _ := pannello(t, false)
	if global.Config.App.WebSso {
		t.Fatal("conf/config.yaml accende app.web-sso")
	}

	req := httptest.NewRequest("POST", "/api/oidc/auth", strings.NewReader(`{"op":"webauth","id":"999000111","uuid":"dXVpZA=="}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en")
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != 400 || rec.Body.String() != `{"error":"Config not found."}` {
		t.Errorf("POST /api/oidc/auth, op webauth: %d %s, atteso 400 {\"error\":\"Config not found.\"}", rec.Code, rec.Body)
	}

	rec = alPannello(g, "/api/admin/oauth/bind", `{"op":"webauth"}`)
	if rec.Code != 400 || rec.Body.String() != `{"error":"Configurazione non trovata."}` {
		t.Errorf("POST /api/admin/oauth/bind, op webauth: %d %s", rec.Code, rec.Body)
	}
}

// TestWebauthConferma prova /api/admin/oauth/confirm, /bindConfirm e /info
// sul router vero. Confirm lega all'utente del pannello solo un login
// webauth non ancora confermato, con app.web-sso acceso: il codice di un
// login OIDC, che deve passare dal provider, riceve la risposta del codice
// scaduto e il dispositivo non riceve il token (UserId resta 0). BindConfirm
// accetta solo un login che il provider ha autenticato (OpenId presente).
// Info non manda verifier PKCE, nonce e dati dell'utente del provider.
func TestWebauthConferma(t *testing.T) {
	g, utente, _ := pannello(t, false)
	if err := service.DB.Create(&model.Oauth{Op: "prova", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto"}).Error; err != nil {
		t.Fatal(err)
	}
	oauth := service.AllService.OauthService
	voce := func(codice, op, action, openId string) *service.OauthCacheItem {
		v := &service.OauthCacheItem{Id: "999000111", Op: op, Action: action, DeviceName: "PC-COLLAUDO", DeviceType: "client",
			OpenId: openId, Username: "utente-del-provider", Email: "utente@esempio.it", Verifier: "verifier-segreto", Nonce: "nonce-segreto"}
		oauth.SetOauthCache(codice, v, 0)
		t.Cleanup(func() { oauth.DeleteOauthCache(codice) })
		return v
	}
	oidc := voce("prova-oidc", "prova", service.OauthActionTypeLogin, "")
	webauth := voce("prova-webauth", model.OauthTypeWebauth, service.OauthActionTypeLogin, "")
	associa := voce("prova-associa", model.OauthTypeWebauth, service.OauthActionTypeBind, "")
	autenticato := voce("prova-autenticato", "prova", service.OauthActionTypeLogin, "sub-1")

	for _, tc := range []struct {
		nome, rotta, codice string
		webSso              bool
		risposta            string
		voce                *service.OauthCacheItem
		legato              bool
	}{
		{"login OIDC", "/api/admin/oauth/confirm", "prova-oidc", true, scaduto, oidc, false},
		{"webauth con web-sso spento", "/api/admin/oauth/confirm", "prova-webauth", false, scaduto, webauth, false},
		{"associazione", "/api/admin/oauth/confirm", "prova-associa", true, scaduto, associa, false},
		{"webauth", "/api/admin/oauth/confirm", "prova-webauth", true, `{"code":0,"message":"success","data":null}`, webauth, true},
		{"webauth gia' confermato", "/api/admin/oauth/confirm", "prova-webauth", true, scaduto, webauth, true},
		{"login OIDC senza provider", "/api/admin/oauth/bindConfirm", "prova-oidc", true, scaduto, oidc, false},
		{"webauth", "/api/admin/oauth/bindConfirm", "prova-associa", true, scaduto, associa, false},
		{"login OIDC autenticato", "/api/admin/oauth/bindConfirm", "prova-autenticato", false, `{"code":0,"message":"success","data":{"device_type":"client"}}`, autenticato, true},
	} {
		global.Config.App.WebSso = tc.webSso
		rec := alPannello(g, tc.rotta, `{"code":"`+tc.codice+`"}`)
		if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
			t.Errorf("%s, %s: %d %s\n want 200 %s", tc.rotta, tc.nome, rec.Code, got, tc.risposta)
		}
		if legato := tc.voce.UserId == utente.Id; legato != tc.legato || (!legato && tc.voce.UserId != 0) {
			t.Errorf("%s, %s: UserId %d, legato atteso %t", tc.rotta, tc.nome, tc.voce.UserId, tc.legato)
		}
	}
	global.Config.App.WebSso = false

	req := httptest.NewRequest("GET", "/api/admin/oauth/info?code=prova-oidc", nil)
	req.Header.Set("api-token", tokenDelPannello)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if got, want := rec.Body.String(), `{"code":0,"message":"success","data":{"device_name":"PC-COLLAUDO","id":"999000111","op":"prova"}}`; got != want {
		t.Errorf("GET /api/admin/oauth/info:\n got  %s\n want %s", got, want)
	}
}
