package http

import (
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
)

// TestLoginOptionsSenzaOpzioni prova sul router vero che /api/login-options,
// senza provider e con app.web-sso spento, risponde ["common-oidc/[]"]: il
// client 1.4.9 legge la lista dopo "common-oidc/" e senza voci non mostra
// pulsanti. Prima rispondeva ["common-oidc/null"], che il client scartava
// solo passando dal catch dell'errore.
func TestLoginOptionsSenzaOpzioni(t *testing.T) {
	g, _, _ := pannello(t, false)
	webSso := global.Config.App.WebSso
	t.Cleanup(func() { global.Config.App.WebSso = webSso })
	global.Config.App.WebSso = false

	rec := richiesta(g, "GET", "/api/login-options", "", "")
	if got, want := rec.Body.String(), `["common-oidc/[]"]`; rec.Code != 200 || got != want {
		t.Errorf("GET /api/login-options: %d %s, attesi 200 %s", rec.Code, got, want)
	}
}
