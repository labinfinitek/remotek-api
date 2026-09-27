package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestCallbackErroreDelProvider prova sul router vero che la pagina di
// /api/oidc/callback, quando il provider risponde 500, dica solo OauthFailed:
// il testo dell'errore del provider va nel log, con metodo e rotta.
func TestCallbackErroreDelProvider(t *testing.T) {
	g, _, registro := pannello(t, false)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "dettaglio interno del provider", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	if err := service.DB.Create(&model.Oauth{Op: "prova", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: provider.URL}).Error; err != nil {
		t.Fatal(err)
	}
	oauth := service.AllService.OauthService
	oauth.SetOauthCache("prova-callback", &service.OauthCacheItem{Op: "prova", Action: service.OauthActionTypeLogin}, 0)
	t.Cleanup(func() { oauth.DeleteOauthCache("prova-callback") })

	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest("GET", "/api/oidc/callback?state=prova-callback&code=x", nil))
	if corpo := rec.Body.String(); !strings.Contains(corpo, "var msg = 'OauthFailed'") || strings.Contains(corpo, "dettaglio interno") {
		t.Errorf("GET /api/oidc/callback, provider in errore: la pagina non dice solo OauthFailed:\n%s", corpo)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "GET /api/oidc/callback: alla pagina va OauthFailed") || !strings.Contains(nelLog, "dettaglio interno del provider") {
		t.Errorf("GET /api/oidc/callback, provider in errore: nel log mancano rotta o errore:\n%s", nelLog)
	}
}
