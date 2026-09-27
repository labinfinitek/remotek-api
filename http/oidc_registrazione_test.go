package http

import (
	"encoding/json"
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
