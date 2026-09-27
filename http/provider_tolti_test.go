package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// providerOidc avvia un provider OIDC finto: discovery, token senza
// id_token per il codice codiceDelProvider e userinfo con l'utente sub-1.
// Restituisce l'issuer.
func providerOidc(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	scrivi := func(w http.ResponseWriter, corpo string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, corpo)
	}
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		scrivi(w, fmt.Sprintf(`{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"userinfo_endpoint":%q,"jwks_uri":%q}`,
			srv.URL, srv.URL+"/auth", srv.URL+"/token", srv.URL+"/userinfo", srv.URL+"/jwks"))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("code") != codiceDelProvider {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		scrivi(w, `{"access_token":"at-1","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			http.Error(w, "token sbagliato", http.StatusUnauthorized)
			return
		}
		scrivi(w, `{"sub":"sub-1","name":"Utente OIDC","email":"utente@esempio.it","email_verified":true,"preferred_username":"utente-oidc"}`)
	})
	return srv.URL
}

// codiceDelProvider e' il codice che il provider finto accetta al token.
const codiceDelProvider = "codice-del-provider"

// richiesta manda al router una richiesta col corpo JSON corpo, l'api-token
// del pannello e, se non vuota, Accept-Language lingua.
func richiesta(g *gin.Engine, metodo, rotta, lingua, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, rotta, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-token", tokenDelPannello)
	if lingua != "" {
		req.Header.Set("Accept-Language", lingua)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestProviderTolti prova sul router vero un database con provider github,
// google e linuxdo di prima (A3) accanto a uno oidc. Le opzioni di login del
// client e del pannello offrono solo quello oidc, che da solo accende
// auto_oidc; un login o un'associazione con l'op di un tipo tolto risponde
// come un op che non esiste (golden oidc-auth-op-sconosciuto); creare o
// modificare dal pannello un provider di un tipo tolto risponde col messaggio
// OauthTypeRemoved, webauth come un tipo sconosciuto; nessuna riga si perde, e
// il pannello mostra e cancella anche i provider dei tipi tolti.
func TestProviderTolti(t *testing.T) {
	g, _, _ := pannello(t, true)
	provider := []*model.Oauth{
		{Op: "github", OauthType: "github", ClientId: "id", ClientSecret: "segreto"},
		{Op: "google", OauthType: "google", ClientId: "id", ClientSecret: "segreto", Issuer: "https://accounts.google.com"},
		{Op: "linuxdo", OauthType: "linuxdo", ClientId: "id", ClientSecret: "segreto"},
		{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: providerOidc(t)},
	}
	if err := service.DB.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	disabilitata := global.Config.App.DisablePwdLogin
	t.Cleanup(func() { global.Config.App.DisablePwdLogin = disabilitata })
	global.Config.App.DisablePwdLogin = true

	for _, tc := range []struct{ rotta, atteso string }{
		{"/api/login-options", `["common-oidc/[{\"name\":\"aziendale\"}]","oidc/aziendale"]`},
		{"/api/admin/login-options", `{"code":0,"message":"success","data":{"auto_oidc":true,"disable_pwd":true,"need_captcha":false,"ops":["aziendale"],"register":false}}`},
	} {
		if rec := richiesta(g, "GET", tc.rotta, "", ""); rec.Code != 200 || rec.Body.String() != tc.atteso {
			t.Errorf("GET %s: %d %s\n want 200 %s", tc.rotta, rec.Code, rec.Body, tc.atteso)
		}
	}

	for _, rotta := range []string{"/api/oidc/auth", "/api/admin/oidc/auth", "/api/admin/oauth/bind"} {
		for _, lingua := range []string{"", "en"} {
			inesistente := richiesta(g, "POST", rotta, lingua, `{"op":"remotek-collaudo-inesistente","id":"999000111"}`)
			if rotta == "/api/oidc/auth" && lingua == "en" && (inesistente.Code != 400 || inesistente.Body.String() != `{"error":"Config not found."}`) {
				t.Fatalf("POST %s, op inesistente: %d %s, non come il golden oidc-auth-op-sconosciuto", rotta, inesistente.Code, inesistente.Body)
			}
			for _, op := range []string{"github", "google", "linuxdo"} {
				rec := richiesta(g, "POST", rotta, lingua, `{"op":"`+op+`","id":"999000111"}`)
				if rec.Code != inesistente.Code || rec.Body.String() != inesistente.Body.String() {
					t.Errorf("POST %s, op %s, Accept-Language %q: %d %s\n want %d %s, come un op che non esiste",
						rotta, op, lingua, rec.Code, rec.Body, inesistente.Code, inesistente.Body)
				}
			}
		}
	}

	const (
		tolto       = `{"code":101,"message":"Questo tipo di provider non è più supportato: usa OIDC.","data":null}`
		sconosciuto = `{"code":101,"message":"Parametri non validi.","data":null}`
		riuscito    = `{"code":0,"message":"success","data":null}`
	)
	id := strconv.FormatUint(uint64(provider[1].Id), 10)
	for _, tc := range []struct{ rotta, corpo, atteso string }{
		{"/api/admin/oauth/create", `{"op":"google","oauth_type":"google","client_id":"id","client_secret":"segreto"}`, tolto},
		{"/api/admin/oauth/create", `{"op":"github-nuovo","oauth_type":"github","client_id":"id","client_secret":"segreto"}`, tolto},
		{"/api/admin/oauth/create", `{"oauth_type":"linuxdo","client_id":"id","client_secret":"segreto"}`, tolto},
		{"/api/admin/oauth/create", `{"op":"webauth","oauth_type":"webauth","client_id":"id","client_secret":"segreto"}`, sconosciuto},
		{"/api/admin/oauth/update", `{"id":` + id + `,"op":"google","oauth_type":"google","client_id":"id","client_secret":"segreto-nuovo","issuer":"https://accounts.google.com"}`, tolto},
		{"/api/admin/oauth/create", `{"op":"google-oidc","oauth_type":"oidc","client_id":"id","client_secret":"segreto","issuer":"https://accounts.google.com"}`, riuscito},
	} {
		if rec := richiesta(g, "POST", tc.rotta, "", tc.corpo); rec.Code != 200 || rec.Body.String() != tc.atteso {
			t.Errorf("POST %s %s: %d %s\n want 200 %s", tc.rotta, tc.corpo, rec.Code, rec.Body, tc.atteso)
		}
	}
	google := &model.Oauth{}
	if err := service.DB.First(google, provider[1].Id).Error; err != nil {
		t.Fatal(err)
	}
	if google.ClientSecret != "segreto" || google.OauthType != "google" {
		t.Errorf("provider google dopo la modifica rifiutata: tipo %q, secret cambiato %t", google.OauthType, google.ClientSecret != "segreto")
	}

	var ops []string
	if err := service.DB.Model(&model.Oauth{}).Order("id").Pluck("op", &ops).Error; err != nil {
		t.Fatal(err)
	}
	if want := []string{"github", "google", "linuxdo", "aziendale", "google-oidc"}; !slices.Equal(ops, want) {
		t.Errorf("provider nel database: %q, attesi %q", ops, want)
	}
	if rec := richiesta(g, "GET", "/api/admin/oauth/list", "", ""); !strings.Contains(rec.Body.String(), `"op":"github"`) || !strings.Contains(rec.Body.String(), `"total":5`) {
		t.Errorf("GET /api/admin/oauth/list: %s", rec.Body)
	}
	github := strconv.FormatUint(uint64(provider[0].Id), 10)
	if rec := richiesta(g, "GET", "/api/admin/oauth/detail/"+github, "", ""); !strings.Contains(rec.Body.String(), `"oauth_type":"github"`) {
		t.Errorf("GET /api/admin/oauth/detail/%s: %s", github, rec.Body)
	}
	if rec := richiesta(g, "POST", "/api/admin/oauth/delete", "", `{"id":`+github+`}`); rec.Body.String() != riuscito {
		t.Errorf("POST /api/admin/oauth/delete del provider github: %s", rec.Body)
	}
	var n int64
	if err := service.DB.Model(&model.Oauth{}).Where("op = ?", "github").Count(&n).Error; err != nil || n != 0 {
		t.Errorf("provider github dopo la cancellazione: %d righe (err %v)", n, err)
	}
}

// TestProviderOidc prova sul router vero che un provider oidc funziona come
// prima, fino al callback: /api/oidc/auth risponde col codice e l'indirizzo
// del provider, il callback scambia il codice, legge l'utente, lo registra
// (auto_register), lo associa al provider e lega il login all'utente.
func TestProviderOidc(t *testing.T) {
	g, _, _ := pannello(t, false)
	issuer := providerOidc(t)
	registra := true
	if err := service.DB.Create(&model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
		Issuer: issuer, AutoRegister: &registra}).Error; err != nil {
		t.Fatal(err)
	}

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
	q := indirizzo.Query()
	if indirizzo.Scheme+"://"+indirizzo.Host+indirizzo.Path != issuer+"/auth" || q.Get("state") != risposta.Code || q.Get("client_id") != "id" ||
		q.Get("scope") != "openid profile email" || q.Get("nonce") == "" ||
		q.Get("redirect_uri") != global.Config.Rustdesk.ApiServer+"/api/oidc/callback" {
		t.Errorf("POST /api/oidc/auth: indirizzo del provider %s", risposta.Url)
	}

	rec = richiesta(g, "GET", "/api/oidc/callback?state="+risposta.Code+"&code="+codiceDelProvider, "", "")
	if !strings.Contains(rec.Body.String(), "var msg = 'OauthSuccess'") {
		t.Fatalf("GET /api/oidc/callback: %d %s", rec.Code, rec.Body)
	}
	ut := &model.UserThird{}
	if err := service.DB.Where("op = ? AND open_id = ?", "aziendale", "sub-1").First(ut).Error; err != nil {
		t.Fatalf("associazione dell'utente del provider: %v", err)
	}
	u := &model.User{}
	if err := service.DB.First(u, ut.UserId).Error; err != nil {
		t.Fatal(err)
	}
	voce := service.AllService.OauthService.GetOauthCache(risposta.Code)
	if u.Username != "utente-oidc" || u.Email != "utente@esempio.it" || ut.OauthType != model.OauthTypeOidc || voce == nil || voce.UserId != u.Id {
		t.Errorf("dopo il callback: utente %q, email %q, tipo %q, login in cache %+v", u.Username, u.Email, ut.OauthType, voce)
	}
}

// TestLoginOptionsDatabaseInErrore prova che le opzioni di login, se l'elenco
// dei provider non si legge, rispondono con l'errore di sistema, nella forma
// delle risposte d'errore del client e del pannello, e l'errore va nel log.
func TestLoginOptionsDatabaseInErrore(t *testing.T) {
	g, _, registro := pannello(t, false)
	if err := service.DB.Migrator().DropTable(&model.Oauth{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ rotta, atteso string }{
		{"/api/login-options", `{"error":"Errore di sistema."}`},
		{"/api/admin/login-options", `{"code":101,"message":"Errore di sistema.","data":null}`},
	} {
		registro.Reset()
		rec := richiesta(g, "GET", tc.rotta, "", "")
		if rec.Body.String() != tc.atteso {
			t.Errorf("GET %s senza la tabella oauths: %d %s\n want %s", tc.rotta, rec.Code, rec.Body, tc.atteso)
		}
		if nelLog := registro.String(); !strings.Contains(nelLog, "GET "+tc.rotta+": ") || !strings.Contains(nelLog, "no such table: oauths") {
			t.Errorf("GET %s senza la tabella oauths, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
		}
	}
}
