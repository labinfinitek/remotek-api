package http

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestSegretoDelProvider prova sul router vero che il segreto del client
// OIDC non torna nella lista e nel dettaglio del provider, dove c'e'
// has_client_secret; che un aggiornamento col segreto vuoto o di soli spazi
// tiene quello salvato e uno con un segreto nuovo lo cambia; che la
// creazione senza segreto, o con un segreto di soli spazi, si rifiuta. Prima
// lista e dettaglio rimandavano il segreto in chiaro e l'aggiornamento senza
// segreto si rifiutava; un segreto di spazi sostituiva quello salvato.
func TestSegretoDelProvider(t *testing.T) {
	g, _, _ := pannello(t, true)
	p := &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto-salvato", Issuer: "https://idp.example"}
	crea(t, p)
	id := strconv.FormatUint(uint64(p.Id), 10)

	for _, rotta := range []string{"/api/admin/oauth/list", "/api/admin/oauth/detail/" + id} {
		rec := richiesta(g, "GET", rotta, "", "")
		if corpo := rec.Body.String(); rec.Code != 200 || strings.Contains(corpo, "segreto-salvato") ||
			strings.Contains(corpo, `"client_secret"`) || !strings.Contains(corpo, `"has_client_secret":true`) {
			t.Errorf("GET %s: %d %s, atteso senza il segreto e con has_client_secret true", rotta, rec.Code, corpo)
		}
	}

	segreto := func() string {
		t.Helper()
		var s string
		if err := service.DB.Raw("SELECT client_secret FROM oauths WHERE id = ?", p.Id).Scan(&s).Error; err != nil {
			t.Fatal(err)
		}
		return s
	}
	const riuscito = `{"code":0,"message":"success","data":null}`
	modifica := `{"id":` + id + `,"op":"aziendale","oauth_type":"oidc","client_id":"id-nuovo","issuer":"https://idp.example"`
	for _, vuoto := range []string{"", "   "} {
		if rec := richiesta(g, "POST", "/api/admin/oauth/update", "", modifica+`,"client_secret":"`+vuoto+`"}`); rec.Body.String() != riuscito {
			t.Errorf("aggiornamento col segreto %q: %d %s", vuoto, rec.Code, rec.Body)
		}
		if s := segreto(); s != "segreto-salvato" {
			t.Errorf("dopo l'aggiornamento col segreto %q il segreto e' cambiato in %q", vuoto, s)
		}
	}
	if rec := richiesta(g, "POST", "/api/admin/oauth/update", "", modifica+`,"client_secret":"segreto-nuovo"}`); rec.Body.String() != riuscito {
		t.Errorf("aggiornamento col segreto nuovo: %d %s", rec.Code, rec.Body)
	}
	if segreto() != "segreto-nuovo" {
		t.Errorf("dopo l'aggiornamento col segreto nuovo il segreto non e' cambiato")
	}

	for _, vuoto := range []string{``, `,"client_secret":"   "`} {
		rec := richiesta(g, "POST", "/api/admin/oauth/create", "", `{"op":"secondo","oauth_type":"oidc","client_id":"id","issuer":"https://idp.example"`+vuoto+`}`)
		if rec.Body.String() != `{"code":101,"message":"ClientSecret è un campo obbligatorio","data":null}` || righe(t, "oauths") != 1 {
			t.Errorf("creazione con %q: %d %s, %d provider; atteso il rifiuto", vuoto, rec.Code, rec.Body, righe(t, "oauths"))
		}
	}
}
