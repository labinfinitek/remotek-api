package http

import (
	"encoding/json"
	"strings"
	"testing"

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
