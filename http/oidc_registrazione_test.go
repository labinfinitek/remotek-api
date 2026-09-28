package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestRegistrazioneOidcUtenteEsistente prova sul router vero il login OIDC
// con autoregistrazione di sub-1 (utente-oidc, utente@esempio.it) quando nel
// database c'e' gia' qualcosa di suo. Con l'email di un utente locale il
// login si lega a quell'utente e lo associa al provider; se l'associazione
// non si salva la pagina dice OauthRegisterFailed e il login non si lega a
// nessuno. Se l'associazione o l'utente con l'email non si leggono, o
// l'associazione e' di un utente che non c'e', la pagina dice OauthFailed e
// non nasce nessun utente. Prima nel secondo caso il login riusciva senza
// associazione, nel terzo e nel quarto nasceva un utente doppio (un errore
// di lettura valeva "non trovato"), nel quinto la pagina diceva successo e
// il client aspettava un login che non arrivava.
func TestRegistrazioneOidcUtenteEsistente(t *testing.T) {
	for _, tc := range []struct {
		nome, messaggio, nelLog string
		// prepara crea nel database quello che il caso vuole e restituisce
		// l'utente a cui il login si deve legare, 0 se a nessuno.
		prepara func(t *testing.T) uint
	}{
		{"email di un utente locale", "OauthSuccess", "", func(t *testing.T) uint {
			locale := &model.User{Username: "locale", Email: "utente@esempio.it"}
			crea(t, locale)
			return locale.Id
		}},
		{"associazione per email rifiutata", "OauthRegisterFailed", "rifiutato dal test", func(t *testing.T) uint {
			crea(t, &model.User{Username: "locale", Email: "utente@esempio.it"})
			rifiuta(t, "BEFORE INSERT ON user_thirds")
			return 0
		}},
		{"associazione non letta", "OauthFailed", "lettura rifiutata dal test", func(t *testing.T) uint {
			legato := &model.User{Username: "utente-oidc"}
			crea(t, legato)
			crea(t, &model.UserThird{UserId: legato.Id, Op: "aziendale", OauthType: model.OauthTypeOidc, OauthUser: model.OauthUser{OpenId: "sub-1"}})
			rifiutaLetture(t, "user_thirds", "")
			return 0
		}},
		{"email non letta", "OauthFailed", "lettura rifiutata dal test", func(t *testing.T) uint {
			crea(t, &model.User{Username: "locale", Email: "utente@esempio.it"})
			rifiutaLetture(t, "users", "email")
			return 0
		}},
		{"associazione senza utente", "OauthFailed", "record not found", func(t *testing.T) uint {
			crea(t, &model.UserThird{UserId: 999999, Op: "aziendale", OauthType: model.OauthTypeOidc, OauthUser: model.OauthUser{OpenId: "sub-1"}})
			return 0
		}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			registra := true
			crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
				Issuer: providerOidc(t), AutoRegister: &registra})
			atteso := tc.prepara(t)
			utenti := righe(t, "users")

			rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
			var risposta struct{ Code string }
			if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 {
				t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
			}
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(risposta.Code) })

			rec = richiesta(g, "GET", "/api/oidc/callback?state="+risposta.Code+"&code="+codiceDelProvider, "", "")
			if !strings.Contains(rec.Body.String(), "var msg = '"+tc.messaggio+"'") {
				t.Errorf("GET /api/oidc/callback: %d, atteso il messaggio %s\n%s", rec.Code, tc.messaggio, rec.Body)
			}
			if voce := service.AllService.OauthService.GetOauthCache(risposta.Code); voce == nil || voce.UserId != atteso {
				t.Errorf("login in cache dopo il callback: %+v, atteso legato all'utente %d", voce, atteso)
			}
			if tc.nelLog != "" && !strings.Contains(registro.String(), tc.nelLog) {
				t.Errorf("nel log manca %q:\n%s", tc.nelLog, registro)
			}
			if n := righe(t, "users"); n != utenti {
				t.Errorf("utenti dopo il callback: %d, erano %d", n, utenti)
			}
		})
	}
}

// crea salva m nel database dei servizi.
func crea(t *testing.T, m any) {
	t.Helper()
	if err := service.DB.Create(m).Error; err != nil {
		t.Fatal(err)
	}
}

// TestAutoregistrazioneNonIndicata prova sul router vero il login OIDC di un
// account senza utente con un provider salvato senza auto_register, NULL nel
// database: l'autoregistrazione vale spenta, il callback rimanda ad
// associare l'account e non crea ne' utenti ne' associazioni. Prima
// dereferenziava nil e andava in panic (500).
func TestAutoregistrazioneNonIndicata(t *testing.T) {
	g, _, _ := pannello(t, false)
	crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
		Issuer: providerOidc(t)})
	if got := autoregistrazione(t); got != "NULL" {
		t.Fatalf("auto_register del provider di prova: %s, atteso NULL", got)
	}
	utenti, associazioni := righe(t, "users"), righe(t, "user_thirds")

	rec := richiesta(g, "POST", "/api/oidc/auth", "", `{"op":"aziendale","id":"999000111","uuid":"dXVpZA==","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
	var risposta struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 {
		t.Fatalf("POST /api/oidc/auth: %d %s (%v)", rec.Code, rec.Body, err)
	}
	t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache(risposta.Code) })

	rec = richiesta(g, "GET", "/api/oidc/callback?state="+risposta.Code+"&code="+codiceDelProvider, "", "")
	if got, want := rec.Header().Get("Location"), "/_admin/#/oauth/bind/"+risposta.Code; rec.Code != http.StatusFound || got != want {
		t.Errorf("GET /api/oidc/callback: %d verso %q, attesi 302 verso %q\n%s", rec.Code, got, want, rec.Body)
	}
	if voce := service.AllService.OauthService.GetOauthCache(risposta.Code); voce == nil || voce.OpenId != "sub-1" || voce.UserId != 0 {
		t.Errorf("login in cache dopo il callback: %+v, atteso l'account sub-1 da associare", voce)
	}
	if u, a := righe(t, "users"), righe(t, "user_thirds"); u != utenti || a != associazioni {
		t.Errorf("dopo il callback %d utenti e %d associazioni, erano %d e %d", u, a, utenti, associazioni)
	}
}

// TestProviderSenzaAutoregistrazione prova sul router vero che il pannello,
// quando crea o aggiorna un provider senza auto_register, salva spenta
// l'autoregistrazione, come fa per pkce_enable. Prima la creazione lasciava
// NULL, e l'aggiornamento il valore che c'era.
func TestProviderSenzaAutoregistrazione(t *testing.T) {
	g, _, _ := pannello(t, true)
	const successo = `{"code":0,"message":"success","data":null}`
	const campi = `"op":"aziendale","oauth_type":"oidc","issuer":"https://idp.esempio.it","client_id":"id","client_secret":"segreto"`

	if rec := alPannello(g, "/api/admin/oauth/create", "{"+campi+"}"); rec.Body.String() != successo {
		t.Fatalf("POST /api/admin/oauth/create: %d %s", rec.Code, rec.Body)
	}
	if got := autoregistrazione(t); got != "0" {
		t.Errorf("auto_register dopo la creazione: %s, atteso 0", got)
	}

	var id uint
	if err := service.DB.Raw("SELECT id FROM oauths").Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.DB.Exec("UPDATE oauths SET auto_register = 1").Error; err != nil {
		t.Fatal(err)
	}
	if rec := alPannello(g, "/api/admin/oauth/update", fmt.Sprintf(`{"id":%d,%s}`, id, campi)); rec.Body.String() != successo {
		t.Fatalf("POST /api/admin/oauth/update: %d %s", rec.Code, rec.Body)
	}
	if got := autoregistrazione(t); got != "0" {
		t.Errorf("auto_register dopo una modifica senza il campo: %s, atteso 0", got)
	}
}

// autoregistrazione legge con Raw auto_register dell'unico provider: "0",
// "1" o "NULL".
func autoregistrazione(t *testing.T) string {
	t.Helper()
	var v string
	if err := service.DB.Raw("SELECT COALESCE(CAST(auto_register AS TEXT), 'NULL') FROM oauths").Scan(&v).Error; err != nil {
		t.Fatal(err)
	}
	return v
}
