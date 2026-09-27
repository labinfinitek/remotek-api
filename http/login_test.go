package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

const passwordDiProva = "password-di-prova"

// preparaLogin da' all'utente di pannello la password passwordDiProva, crea
// le tabelle che il login scrive (il registro degli accessi e i
// dispositivi) e da' ai servizi un Jwt senza chiave, che vuol dire token
// casuali come senza jwt.key: pannello non ne passa nessuno.
func preparaLogin(t *testing.T, utente *model.User) {
	t.Helper()
	precJwt := service.Jwt
	service.Jwt = &jwt.Jwt{}
	t.Cleanup(func() { service.Jwt = precJwt })
	hash, err := utils.EncryptPassword(passwordDiProva)
	if err == nil {
		err = service.DB.Model(utente).Update("password", hash).Error
	}
	if err == nil {
		err = service.DB.AutoMigrate(&model.LoginLog{}, &model.Peer{})
	}
	if err != nil {
		t.Fatal(err)
	}
}

// TestLoginDelClient prova sul router vero il login del client (POST
// /api/login) quando una scrittura non riesce. Se il token di sessione o il
// registro degli accessi non si salvano, la risposta e' 400 "Operazione non
// riuscita.", l'errore va nel log e dei due non resta niente: prima la
// risposta dava un token che il database non aveva, o un login fuori dal
// registro. Se il dispositivo non si lega all'utente il login vale lo stesso
// e l'errore va nel log, dove prima non arrivava.
func TestLoginDelClient(t *testing.T) {
	for _, tc := range []struct {
		nome, ostacolo string // ostacolo: la scrittura che un trigger rifiuta
		entra, legato  bool
	}{
		{"senza ostacoli", "", true, true},
		{"token rifiutato", "BEFORE INSERT ON user_tokens", false, false},
		{"registro rifiutato", "BEFORE INSERT ON login_logs", false, false},
		{"dispositivo non legato", "BEFORE UPDATE ON peers", true, false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			preparaLogin(t, utente)
			crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="})
			if tc.ostacolo != "" {
				rifiuta(t, tc.ostacolo)
			}
			scadenza := unaConnessione(t)

			rec := richiesta(g, "POST", "/api/login", "", `{"username":"prova","password":"`+passwordDiProva+`","id":"999000111","uuid":"dXVpZA==",`+
				`"autoLogin":true,"type":"account","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`)
			if scadenza.Err() != nil {
				t.Fatalf("POST /api/login: fermo per 5 secondi ad aspettare la connessione del database (stallo); risposta %d %s", rec.Code, rec.Body)
			}
			token, registrati := int64(1), int64(0) // senza login resta solo il token del pannello
			if tc.entra {
				var risposta struct {
					AccessToken string `json:"access_token"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 || risposta.AccessToken == "" {
					t.Fatalf("POST /api/login: %d %s (%v), atteso il token", rec.Code, rec.Body, err)
				}
				var n int64
				if err := service.DB.Raw("SELECT count(*) FROM user_tokens WHERE token = ?", risposta.AccessToken).Scan(&n).Error; err != nil || n != 1 {
					t.Errorf("token della risposta nel database: %d (err %v), atteso 1", n, err)
				}
				token, registrati = 2, 1
			} else if got, want := rec.Body.String(), `{"error":"Operazione non riuscita."}`; rec.Code != 400 || got != want {
				t.Errorf("POST /api/login: %d %s, attesi 400 e %s", rec.Code, got, want)
			}
			if n := righe(t, "user_tokens"); n != token {
				t.Errorf("token dopo il login: %d, attesi %d", n, token)
			}
			if n := righe(t, "login_logs"); n != registrati {
				t.Errorf("righe del registro degli accessi dopo il login: %d, attese %d", n, registrati)
			}
			var proprietario uint
			if err := service.DB.Raw("SELECT user_id FROM peers").Scan(&proprietario).Error; err != nil {
				t.Fatal(err)
			}
			if legato := proprietario == utente.Id; legato != tc.legato {
				t.Errorf("dispositivo dell'utente %d dopo il login, legato all'utente del login: %t, atteso %t", proprietario, legato, tc.legato)
			}
			if tc.ostacolo != "" && !strings.Contains(registro.String(), "rifiutato dal test") {
				t.Errorf("l'errore del database non e' nel log:\n%s", registro)
			}
		})
	}
}

// TestLoginSenzaToken prova sulle altre tre rotte che fanno entrare un
// utente (login OIDC del client, login e registrazione del pannello) che, se
// il token di sessione non si salva, la risposta e' d'errore e l'errore va
// nel log: prima davano un token che il database non aveva.
func TestLoginSenzaToken(t *testing.T) {
	password := strings.Repeat("p", 15)
	for _, tc := range []struct {
		nome, metodo, rotta, corpo, risposta string
		prepara                              func(t *testing.T, utente *model.User)
	}{
		{"login OIDC del client", "GET", "/api/oidc/auth-query?code=login-oidc&id=999000111&uuid=dXVpZA==", "",
			`{"error":"Accesso non riuscito."}`, func(t *testing.T, utente *model.User) {
				service.AllService.OauthService.SetOauthCache("login-oidc", &service.OauthCacheItem{UserId: utente.Id, Action: service.OauthActionTypeLogin}, 0)
				t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("login-oidc") })
			}},
		{"login del pannello", "POST", "/api/admin/login", `{"username":"prova","password":"` + passwordDiProva + `"}`,
			`{"code":101,"message":"Operazione non riuscita.","data":null}`, func(*testing.T, *model.User) {}},
		{"registrazione dal pannello", "POST", "/api/admin/user/register", `{"username":"nuovo","email":"","password":"` + password + `","confirm_password":"` + password + `"}`,
			`{"code":101,"message":"Operazione non riuscita.","data":null}`, func(*testing.T, *model.User) { global.Config.App.Register = true }},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			preparaLogin(t, utente)
			tc.prepara(t, utente)
			rifiuta(t, "BEFORE INSERT ON user_tokens")

			rec := richiesta(g, tc.metodo, tc.rotta, "", tc.corpo)
			if got := rec.Body.String(); got != tc.risposta {
				t.Errorf("%s %s: %d %s, atteso %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
			}
			if !strings.Contains(registro.String(), "rifiutato dal test") {
				t.Errorf("%s %s: l'errore del database non e' nel log:\n%s", tc.metodo, tc.rotta, registro)
			}
			if n := righe(t, "user_tokens"); n != 1 {
				t.Errorf("token dopo il login: %d, atteso 1, quello del pannello", n)
			}
		})
	}
}

// TestLogoutDispositivo prova sul router vero il logout del pannello
// (POST /api/admin/logout) col token di un dispositivo: senza ostacoli
// cancella il token e scollega il dispositivo dall'utente. Se il token non
// si legge o il dispositivo non si scollega, la risposta e' OperationFailed
// e l'errore va nel log: prima il primo errore saltava lo scollegamento, il
// secondo si perdeva, e in entrambi i casi la risposta era successo col
// dispositivo ancora dell'utente.
func TestLogoutDispositivo(t *testing.T) {
	const tokenDelDispositivo = "token-del-dispositivo"
	for _, tc := range []struct {
		nome, rifiuto string
		ostacolo      func(t *testing.T)
	}{
		{"senza ostacoli", "", func(*testing.T) {}},
		{"token non letto", "lettura rifiutata dal test", func(t *testing.T) { rifiutaLetture(t, "user_tokens", "user_id") }},
		{"dispositivo non scollegato", "rifiutato dal test", func(t *testing.T) { rifiuta(t, "BEFORE UPDATE ON peers") }},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA==", UserId: utente.Id})
			crea(t, &model.UserToken{UserId: utente.Id, Token: tokenDelDispositivo, DeviceUuid: "dXVpZA==", ExpiredAt: time.Now().Add(time.Hour).Unix()})
			tc.ostacolo(t)

			rec := conToken(g, "POST", "/api/admin/logout", tokenDelDispositivo)
			risposta, token, proprietario := `{"code":0,"message":"success","data":null}`, int64(1), uint(0)
			if tc.rifiuto != "" {
				// Il token non si legge: niente si cancella. Il dispositivo
				// non si scollega: il token non c'e' gia' piu'.
				risposta, proprietario = `{"code":101,"message":"Operazione non riuscita.","data":null}`, utente.Id
				if tc.nome == "token non letto" {
					token = 2
				}
				if nelLog := registro.String(); !strings.Contains(nelLog, "POST /api/admin/logout: ") || !strings.Contains(nelLog, tc.rifiuto) {
					t.Errorf("POST /api/admin/logout, nel log mancano rotta o errore:\n%s", nelLog)
				}
			}
			if got := rec.Body.String(); rec.Code != 200 || got != risposta {
				t.Errorf("POST /api/admin/logout: stato %d\n got  %s\n want %s", rec.Code, got, risposta)
			}
			if n := righe(t, "user_tokens"); n != token {
				t.Errorf("token dopo il logout: %d, attesi %d", n, token)
			}
			var got uint
			if err := service.DB.Raw("SELECT user_id FROM peers").Scan(&got).Error; err != nil {
				t.Fatal(err)
			}
			if got != proprietario {
				t.Errorf("dispositivo dopo il logout: dell'utente %d, atteso %d", got, proprietario)
			}
		})
	}
}

// TestRinnovoToken prova sul router vero il rinnovo della scadenza del token
// di sessione, che le rotte del client e del pannello fanno quando al token
// manca meno di un terzo della durata: senza ostacoli la scadenza si
// allunga; se non si salva la risposta e' quella di sempre, il token vale
// fino alla scadenza di prima e l'errore va nel log, con metodo e rotta.
// Prima si perdeva.
func TestRinnovoToken(t *testing.T) {
	for _, tc := range []struct{ nome, ostacolo string }{
		{"senza ostacoli", ""},
		{"scadenza rifiutata", "BEFORE UPDATE ON user_tokens"},
	} {
		for _, r := range []struct{ metodo, rotta, intestazione, valore string }{
			{"POST", "/api/currentUser", "Authorization", "Bearer " + tokenDelPannello},
			{"GET", "/api/admin/user/current", "api-token", tokenDelPannello},
		} {
			t.Run(tc.nome+" "+r.rotta, func(t *testing.T) {
				g, _, registro := pannello(t, false)
				precJwt := global.Jwt
				global.Jwt = jwt.NewJwt("", time.Hour) // senza chiave: RustAuth cerca il token nel database
				t.Cleanup(func() { global.Jwt = precJwt })
				if tc.ostacolo != "" {
					rifiuta(t, tc.ostacolo)
				}
				req := httptest.NewRequest(r.metodo, r.rotta, nil)
				req.Header.Set(r.intestazione, r.valore)
				rec := httptest.NewRecorder()
				g.ServeHTTP(rec, req)

				if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"prova"`) {
					t.Errorf("%s %s: %d %s, attesi 200 e l'utente", r.metodo, r.rotta, rec.Code, rec.Body)
				}
				var scadenza int64
				if err := service.DB.Raw("SELECT expired_at FROM user_tokens").Scan(&scadenza).Error; err != nil {
					t.Fatal(err)
				}
				// pannello crea il token con 24 ore, token-expire e' 168h.
				if rinnovato := scadenza > time.Now().Add(100*time.Hour).Unix(); rinnovato != (tc.ostacolo == "") {
					t.Errorf("%s %s: scadenza rinnovata %t, attesa %t", r.metodo, r.rotta, rinnovato, tc.ostacolo == "")
				}
				if nelLog := registro.String(); tc.ostacolo != "" && (!strings.Contains(nelLog, r.metodo+" "+r.rotta+": ") || !strings.Contains(nelLog, "rifiutato dal test")) {
					t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", r.metodo, r.rotta, nelLog)
				}
			})
		}
	}
}
