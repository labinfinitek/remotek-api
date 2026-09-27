package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/lejianwen/rustdesk-api/v2/global"
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

// assegnazione legge lo script di /api/oidc/msg: una o due assegnazioni
// ";nome = valore;".
var assegnazione = regexp.MustCompile(`^;(title|msg) = (.*);$`)

// traduzioniDaMsg chiede a /api/oidc/msg la traduzione di id come msg, e
// restituisce il valore assegnato riletto con json.Unmarshal.
func traduzioniDaMsg(t *testing.T, g http.Handler, lingua, id string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest("GET", "/api/oidc/msg?lang="+lingua+"&msg="+url.QueryEscape(id), nil))
	m := assegnazione.FindStringSubmatch(rec.Body.String())
	if m == nil || m[1] != "msg" {
		t.Fatalf("/api/oidc/msg, %s: script inatteso %q", id, rec.Body.String())
	}
	var valore string
	if err := json.Unmarshal([]byte(m[2]), &valore); err != nil {
		t.Fatalf("/api/oidc/msg, %s: %q non e' un letterale JSON: %v", id, m[2], err)
	}
	return valore
}

// TestMessaggioConApostrofo prova /api/oidc/msg con gli ID la cui traduzione
// italiana ha un apostrofo, che tra apici chiudeva la stringa JavaScript: il
// valore assegnato, riletto come JSON, e' la traduzione esatta. Il titolo
// esce con la stessa forma.
func TestMessaggioConApostrofo(t *testing.T) {
	g, _, _ := pannello(t, false)
	localizer := global.Localizer("it")
	for _, id := range []string{"DecodeOauthUserInfoError", "GetOauthUserInfoError", "ItemExists", "LdapBindServiceFailed",
		"LdapCreateUserFailed", "LdapToLocalUserFailed", "MailNotMatch", "PwdLoginDisabled"} {
		atteso, err := localizer.LocalizeMessage(&i18n.Message{ID: id})
		if err != nil || !strings.Contains(atteso, "'") {
			t.Fatalf("%s: traduzione %q senza apostrofo (err %v)", id, atteso, err)
		}
		if got := traduzioniDaMsg(t, g, "it", id); got != atteso {
			t.Errorf("/api/oidc/msg, %s: %q, atteso %q", id, got, atteso)
		}
	}

	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest("GET", "/api/oidc/msg?lang=it&title=OauthFailed&msg=ItemExists", nil))
	if got, want := rec.Body.String(), `;title = "Autorizzazione OAuth non riuscita.";;msg = "L'elemento esiste già.";`; got != want {
		t.Errorf("/api/oidc/msg con titolo:\n got  %s\n want %s", got, want)
	}
}

// TestCallbackSenzaState prova il giro della pagina di /api/oidc/callback
// senza state: l'ID che la pagina manda a /api/oidc/msg ha una traduzione
// completa, senza il segnaposto di ParamIsEmpty, che usciva come "Il campo
// <no value> è vuoto.".
func TestCallbackSenzaState(t *testing.T) {
	g, _, _ := pannello(t, false)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest("GET", "/api/oidc/callback?code=x", nil))
	m := regexp.MustCompile(`var msg = '([A-Za-z]+)'`).FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("GET /api/oidc/callback senza state: manca l'ID del messaggio:\n%s", rec.Body.String())
	}
	for lingua, atteso := range map[string]string{
		"it": "Il provider OAuth non ha restituito lo stato del login: ripeti il login.",
		"en": "The OAuth provider did not return the login state: start the login again.",
	} {
		if got := traduzioniDaMsg(t, g, lingua, m[1]); got != atteso {
			t.Errorf("GET /api/oidc/callback senza state, %s: la pagina mostra %q, atteso %q", lingua, got, atteso)
		}
	}
}

// TestPannelloOidcSenzaTestoInterno prova sul router vero i due ErrorErr del
// pannello su /api/admin/oidc/auth (login) e /api/admin/oauth/bind
// (associazione) con un provider OIDC che risponde 500: la risposta e' 400
// col solo messaggio di SystemError, l'errore del provider va nel log con
// metodo e rotta.
func TestPannelloOidcSenzaTestoInterno(t *testing.T) {
	g, _, registro := pannello(t, false)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "dettaglio interno del provider", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	if err := service.DB.Create(&model.Oauth{Op: "prova", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: provider.URL}).Error; err != nil {
		t.Fatal(err)
	}
	for _, rotta := range []string{"/api/admin/oidc/auth", "/api/admin/oauth/bind"} {
		registro.Reset()
		rec := alPannello(g, rotta, `{"op":"prova"}`)
		if rec.Code != 400 || rec.Body.String() != `{"error":"Errore di sistema."}` {
			t.Errorf("POST %s: %d %s, atteso 400 {\"error\":\"Errore di sistema.\"}", rotta, rec.Code, rec.Body)
		}
		if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+rotta+": al client va SystemError") || !strings.Contains(nelLog, "dettaglio interno del provider") {
			t.Errorf("POST %s: nel log mancano rotta o errore:\n%s", rotta, nelLog)
		}
	}
}
