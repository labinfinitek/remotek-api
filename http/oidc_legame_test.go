package http

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// associazione salva l'associazione di userId all'account sub del provider
// aziendale.
func associazione(t *testing.T, userId uint, sub string) {
	t.Helper()
	crea(t, &model.UserThird{UserId: userId, Op: "aziendale", OauthType: model.OauthTypeOidc, OauthUser: model.OauthUser{OpenId: sub}})
}

// testiDelLegame sono i messaggi dei rifiuti dell'associazione, per
// Accept-Language: vuoto (italiano) ed en.
var testiDelLegame = map[string]map[string]string{
	"OauthHasBindOtherUser": {"": "Questo account OIDC è già associato a un altro utente.", "en": "This OIDC account is already bound to another user."},                      //nolint:misspell // italiano
	"OauthUserHasBind":      {"": "Questo utente ha già un account associato a questo provider OIDC.", "en": "This user already has an account bound to this OIDC provider."}, //nolint:misspell // italiano
}

// TestAssociazioneGiaPresente prova sul router vero l'associazione al
// provider dal pannello (bindConfirm) e dal callback di un'associazione
// chiesta dal profilo: sub-1 gia' legato a un altro utente risponde
// OauthHasBindOtherUser, un utente che ha gia' un account del provider
// OauthUserHasBind, e nessuna associazione nasce. Prima bindConfirm le
// scriveva tutte e due, il callback la seconda. Anche la richiesta di
// associazione dal profilo (bind) risponde OauthUserHasBind a chi ha gia' un
// account del provider; prima diceva OauthHasBindOtherUser.
func TestAssociazioneGiaPresente(t *testing.T) {
	for _, tc := range []struct {
		nome, rotta, messaggio string
		prepara                func(t *testing.T, utente *model.User)
	}{
		{"bindConfirm, sub di un altro", "bindConfirm", "OauthHasBindOtherUser", func(t *testing.T, _ *model.User) {
			altro := &model.User{Username: "altro"}
			crea(t, altro)
			associazione(t, altro.Id, "sub-1")
		}},
		{"bindConfirm, utente gia' legato", "bindConfirm", "OauthUserHasBind", func(t *testing.T, utente *model.User) {
			associazione(t, utente.Id, "sub-9")
		}},
		{"callback, sub di un altro", "callback", "OauthHasBindOtherUser", func(t *testing.T, _ *model.User) {
			altro := &model.User{Username: "altro"}
			crea(t, altro)
			associazione(t, altro.Id, "sub-1")
		}},
		{"callback, utente gia' legato", "callback", "OauthUserHasBind", func(t *testing.T, utente *model.User) {
			associazione(t, utente.Id, "sub-9")
		}},
		{"bind, utente gia' legato", "bind", "OauthUserHasBind", func(t *testing.T, utente *model.User) {
			associazione(t, utente.Id, "sub-9")
		}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, _ := pannello(t, false)
			crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto", Issuer: providerOidc(t)})
			tc.prepara(t, utente)
			prima := righe(t, "user_thirds")
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("prova-legame") })

			switch tc.rotta {
			case "bind":
				for lingua, testo := range testiDelLegame[tc.messaggio] {
					rec := richiesta(g, "POST", "/api/admin/oauth/bind", lingua, `{"op":"aziendale"}`)
					if atteso := `{"code":101,"message":"` + testo + `","data":null}`; rec.Body.String() != atteso {
						t.Errorf("POST bind, lingua %q: %d %s\n want %s", lingua, rec.Code, rec.Body, atteso)
					}
				}
			case "bindConfirm":
				service.AllService.OauthService.SetOauthCache("prova-legame", &service.OauthCacheItem{
					Op: "aziendale", Action: service.OauthActionTypeLogin, OpenId: "sub-1"}, 0)
				for lingua, testo := range testiDelLegame[tc.messaggio] {
					rec := richiesta(g, "POST", "/api/admin/oauth/bindConfirm", lingua, `{"code":"prova-legame"}`)
					if atteso := `{"code":101,"message":"` + testo + `","data":null}`; rec.Body.String() != atteso {
						t.Errorf("POST bindConfirm, lingua %q: %d %s\n want %s", lingua, rec.Code, rec.Body, atteso)
					}
				}
				if voce := service.AllService.OauthService.GetOauthCache("prova-legame"); voce == nil || voce.UserId != 0 {
					t.Errorf("login in cache dopo il rifiuto: %+v, atteso senza utente", voce)
				}
			default:
				service.AllService.OauthService.SetOauthCache("prova-legame", &service.OauthCacheItem{
					Op: "aziendale", Action: service.OauthActionTypeBind, UserId: utente.Id}, 0)
				rec := richiesta(g, "GET", "/api/oidc/callback?state=prova-legame&code="+codiceDelProvider, "", "")
				if !strings.Contains(rec.Body.String(), "var msg = '"+tc.messaggio+"'") {
					t.Errorf("GET /api/oidc/callback: %d, atteso il messaggio %s\n%s", rec.Code, tc.messaggio, rec.Body)
				}
			}
			if n := righe(t, "user_thirds"); n != prima {
				t.Errorf("associazioni dopo il rifiuto: %d, erano %d", n, prima)
			}
		})
	}
}

// TestAuthQueryUtenteDisattivato prova sul router vero che un login OIDC
// gia' confermato dal provider per un utente disattivato non da' il token,
// ne' al client ne' al pannello: la risposta e' UserDisabled nella forma di
// ciascuno, nessun token nasce e il login esce dalla cache. Prima il token
// arrivava.
func TestAuthQueryUtenteDisattivato(t *testing.T) {
	for rotta, atteso := range map[string]string{
		"/api/oidc/auth-query":       `{"error":"Utente disattivato."}`,
		"/api/admin/oidc/auth-query": `{"code":101,"message":"Utente disattivato.","data":null}`,
	} {
		t.Run(rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			preparaLogin(t, utente)
			spento := &model.User{Username: "spento", Status: model.COMMON_STATUS_DISABLED}
			crea(t, spento)
			service.AllService.OauthService.SetOauthCache("login-spento", &service.OauthCacheItem{UserId: spento.Id, Action: service.OauthActionTypeLogin}, 0)
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("login-spento") })

			rec := richiesta(g, "GET", rotta+"?code=login-spento&id=999000111&uuid=dXVpZA==", "", "")
			if rec.Body.String() != atteso {
				t.Errorf("GET %s: %d %s\n want %s", rotta, rec.Code, rec.Body, atteso)
			}
			if n := righe(t, "user_tokens"); n != 1 {
				t.Errorf("token dopo il rifiuto: %d, atteso 1, quello del pannello", n)
			}
			if service.AllService.OauthService.GetOauthCache("login-spento") != nil {
				t.Errorf("il login rifiutato e' ancora nella cache")
			}
			if !strings.Contains(registro.String(), "login OIDC rifiutato all'utente disattivato") {
				t.Errorf("nel log manca il warn:\n%s", registro)
			}
		})
	}
}

// TestAutoregistrazioneEmail prova sul router vero il login OIDC con
// autoregistrazione di sub-1 (utente@esempio.it) senza associazione: l'email
// abbina un utente locale solo se il provider l'ha verificata e l'utente non
// e' un agente ne' disattivato. Altrimenti la pagina dice
// OauthEmailNotMatchable e non nascono associazioni ne' utenti; un'email che
// non e' di nessuno crea un utente nuovo anche se non verificata, come
// prima. Prima l'email abbinava anche nei tre casi rifiutati.
func TestAutoregistrazioneEmail(t *testing.T) {
	for _, tc := range []struct {
		nome          string
		nonVerificata bool
		locale        *model.User // utente con l'email, nil se nessuno
		messaggio     string
	}{
		{"email non verificata", true, &model.User{Username: "locale", Email: "utente@esempio.it", Status: model.COMMON_STATUS_ENABLE}, "OauthEmailNotMatchable"},
		{"email di un agente", false, &model.User{Username: "agente", Email: "utente@esempio.it", Status: model.COMMON_STATUS_ENABLE, AgenteDi: 1}, "OauthEmailNotMatchable"},
		{"email di un disattivato", false, &model.User{Username: "spento", Email: "utente@esempio.it", Status: model.COMMON_STATUS_DISABLED}, "OauthEmailNotMatchable"},
		{"email di nessuno, non verificata", true, nil, "OauthSuccess"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, _ := pannello(t, false)
			registra := true
			crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
				Issuer: providerOidcCon(t, idTokenFinto{emailNonVerificata: tc.nonVerificata}), AutoRegister: &registra})
			if tc.locale != nil {
				crea(t, tc.locale)
			}
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
			nuovi, associazioni := int64(0), int64(0)
			if tc.locale == nil {
				nuovi, associazioni = 1, 1
			}
			if n := righe(t, "users"); n != utenti+nuovi {
				t.Errorf("utenti dopo il callback: %d, attesi %d", n, utenti+nuovi)
			}
			if n := righe(t, "user_thirds"); n != associazioni {
				t.Errorf("associazioni dopo il callback: %d, attese %d", n, associazioni)
			}
		})
	}
}
