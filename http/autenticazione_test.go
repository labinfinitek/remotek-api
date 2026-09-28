package http

// Test delle letture del modulo auth sul router vero: un errore del database
// non vale "non trovato", non fa uscire il tecnico dal client o dal pannello
// e non lo banna.

import (
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// TestRustAuthLetturaFallita prova sul router vero RustAuth, il middleware
// delle rotte autenticate del client, quando il database non legge il token
// o il suo utente: la risposta e' 500 {"error": "Errore di sistema."}, con
// l'errore nel log. Prima era 401, che per il client 1.4.9 e' un logout
// forzato, come su /api/currentUser il 400 (user_model.dart:81-82). Un token
// che non c'e', scaduto, di un utente disabilitato o che non c'e' piu' ha il
// 401 di prima (golden currentUser-dopo-logout).
func TestRustAuthLetturaFallita(t *testing.T) {
	const erroreDiSistema, nonAutorizzato = `{"error":"Errore di sistema."}`, `{"error":"Unauthorized"}`
	scadenza := time.Now().Add(time.Hour).Unix()
	for _, tc := range []struct {
		nome, metodo, rotta, token string
		prepara                    func(t *testing.T, utente *model.User)
		stato                      int
		risposta                   string
	}{
		{"token non letto", "POST", "/api/currentUser", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "user_tokens", "") }, 500, erroreDiSistema},
		{"utente non letto", "POST", "/api/currentUser", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "users", "") }, 500, erroreDiSistema},
		{"utente non letto", "GET", "/api/users", tokenDelPannello,
			func(t *testing.T, _ *model.User) { rifiutaLetture(t, "users", "") }, 500, erroreDiSistema},
		{"token che non c'e'", "POST", "/api/currentUser", "token-che-non-esiste",
			func(*testing.T, *model.User) {}, 401, nonAutorizzato},
		{"token scaduto", "POST", "/api/currentUser", "token-scaduto", func(t *testing.T, utente *model.User) {
			crea(t, &model.UserToken{UserId: utente.Id, Token: "token-scaduto", ExpiredAt: time.Now().Add(-time.Hour).Unix()})
		}, 401, nonAutorizzato},
		{"utente disabilitato", "POST", "/api/currentUser", tokenDelPannello, func(t *testing.T, utente *model.User) {
			if err := service.DB.Model(utente).Update("status", model.COMMON_STATUS_DISABLED).Error; err != nil {
				t.Fatal(err)
			}
		}, 401, nonAutorizzato},
		{"utente che non c'e' piu'", "POST", "/api/currentUser", "token-senza-utente", func(t *testing.T, _ *model.User) {
			crea(t, &model.UserToken{UserId: 999999, Token: "token-senza-utente", ExpiredAt: scadenza})
		}, 401, nonAutorizzato},
	} {
		t.Run(tc.nome+" "+tc.rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			precJwt := global.Jwt
			global.Jwt = jwt.NewJwt("", time.Hour) // senza chiave: RustAuth cerca il token nel database
			t.Cleanup(func() { global.Jwt = precJwt })
			tc.prepara(t, utente)

			req := httptest.NewRequest(tc.metodo, tc.rotta, strings.NewReader(`{"id":"999000111","uuid":"dXVpZA=="}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)
			if got := rec.Body.String(); rec.Code != tc.stato || got != tc.risposta {
				t.Errorf("%s %s: %d %s, attesi %d e %s", tc.metodo, tc.rotta, rec.Code, got, tc.stato, tc.risposta)
			}
			if nelLog := registro.String(); tc.stato == 500 && (!strings.Contains(nelLog, tc.metodo+" "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestBackendUserAuthLetturaFallita prova sul router vero BackendUserAuth, il
// middleware del pannello, e la configurazione del pannello, che legge il
// token da sola, quando il database non legge il token o il suo utente: la
// risposta e' code 101 "Errore di sistema.", con l'errore nel log. Prima il
// middleware rispondeva code 403 NeedLogin, che per il pannello e' un logout
// (request.js), e la configurazione rispondeva come senza login. Un token che
// non c'e' ha le risposte di prima.
func TestBackendUserAuthLetturaFallita(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	for _, tc := range []struct {
		rotta, token, tabella, risposta string
	}{
		{"/api/admin/user/current", tokenDelPannello, "user_tokens", erroreDiSistema},
		{"/api/admin/user/current", tokenDelPannello, "users", erroreDiSistema},
		{"/api/admin/config/admin", tokenDelPannello, "user_tokens", erroreDiSistema},
		{"/api/admin/config/admin", tokenDelPannello, "users", erroreDiSistema},
		{"/api/admin/user/current", "token-che-non-esiste", "", `{"code":403,"message":"NEEDLOGIN","data":null}`},
		{"/api/admin/config/admin", "token-che-non-esiste", "", `{"code":0,"message":"success","data":{"title":"TITOLO"}}`},
	} {
		t.Run(tc.rotta+" "+tc.token+" "+tc.tabella, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			if tc.tabella != "" {
				rifiutaLetture(t, tc.tabella, "")
			}
			rec := conToken(g, "GET", tc.rotta, tc.token)
			accedi, err := global.Localizer("").LocalizeMessage(&i18n.Message{ID: "NeedLogin"})
			if err != nil {
				t.Fatal(err)
			}
			risposta := strings.NewReplacer("TITOLO", global.Config.Admin.Title, "NEEDLOGIN", accedi).Replace(tc.risposta)
			if got := rec.Body.String(); rec.Code != 200 || got != risposta {
				t.Errorf("GET %s: stato %d\n got  %s\n want %s", tc.rotta, rec.Code, got, risposta)
			}
			if nelLog := registro.String(); tc.tabella != "" && (!strings.Contains(nelLog, "GET "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("GET %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}
		})
	}
}

// TestLoginLetturaFallita prova sul router vero il login del client e del
// pannello quando il database non legge l'utente: la risposta e' "Errore di
// sistema." (client 400 {"error": ...}, pannello code 101), con l'errore nel
// log, e il tentativo non conta tra quelli falliti. Col ban a 1 tentativo
// fallito, un secondo login con la password giusta entra. Prima la lettura
// fallita valeva una password sbagliata: il primo login bannava l'IP e il
// secondo riceveva il ban (code 423).
func TestLoginLetturaFallita(t *testing.T) {
	for _, tc := range []struct {
		rotta, corpo, erroreDiSistema, entrato string
	}{
		{"/api/login", `{"username":"prova","password":"` + passwordDiProva + `","id":"999000111","uuid":"dXVpZA==",` +
			`"autoLogin":true,"type":"account","deviceInfo":{"os":"windows","type":"client","name":"PC-COLLAUDO"}}`,
			`{"error":"Errore di sistema."}`, `"access_token":`},
		{"/api/admin/login", `{"username":"prova","password":"` + passwordDiProva + `"}`,
			`{"code":101,"message":"Errore di sistema.","data":null}`, `{"code":0,`},
	} {
		t.Run(tc.rotta, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			preparaLogin(t, utente)
			prec := global.LoginLimiter
			global.LoginLimiter = utils.NewLoginLimiter(utils.SecurityPolicy{CaptchaThreshold: global.Config.App.CaptchaThreshold, BanThreshold: 1})
			t.Cleanup(func() { global.LoginLimiter = prec })
			rifiutaLetture(t, "users", "")

			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if got := rec.Body.String(); got != tc.erroreDiSistema {
				t.Errorf("POST %s con gli utenti non letti: %d %s, atteso %s", tc.rotta, rec.Code, got, tc.erroreDiSistema)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}

			// Il database torna a leggere: il login entra se il primo non
			// ha contato come tentativo fallito.
			if err := service.DB.Callback().Query().Remove("rifiuta_letture_users"); err != nil {
				t.Fatal(err)
			}
			if rec := richiesta(g, "POST", tc.rotta, "", tc.corpo); rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.entrato) {
				t.Errorf("POST %s con la password giusta, dopo la lettura fallita: %d %s, atteso il login", tc.rotta, rec.Code, rec.Body)
			}
		})
	}
}

// idDi restituisce con Raw, che rifiutaLetture non ferma, l'id dell'utente
// username, come testo.
func idDi(t *testing.T, username string) string {
	t.Helper()
	var id uint
	if err := service.DB.Raw("SELECT id FROM users WHERE username = ?", username).Scan(&id).Error; err != nil || id == 0 {
		t.Fatalf("id di %s: %d (%v)", username, id, err)
	}
	return strconv.FormatUint(uint64(id), 10)
}

// TestClientUtentiNonLetti prova sul router vero le rotte del client che,
// dopo RustAuth, leggono gli utenti: se il database non li legge rispondono
// 400 {"error": "Errore di sistema."}, con l'errore nel log, e non 401, che
// per il client e' un logout. Prima /api/users e /api/peers rispondevano 200
// con l'elenco vuoto, le rubriche condivise andavano in panic (500), e una
// rubrica di un altro utente rispondeva "Parametri non validi.".
func TestClientUtentiNonLetti(t *testing.T) {
	for _, tc := range []struct {
		nome, metodo, rotta, dove string // in dove PROPRIETARIO diventa l'id del proprietario
		amministratore            bool   // l'amministratore vede gli utenti del suo gruppo
	}{
		{"utenti del gruppo", "GET", "/api/users", "group_id", true},
		{"dispositivi del gruppo", "GET", "/api/peers", "group_id", true},
		{"proprietari delle rubriche condivise", "POST", "/api/ab/shared/profiles", "id in", false},
		{"proprietario della rubrica", "POST", "/api/ab/peers?ab=CONDIVISA", "id = ? [PROPRIETARIO]", false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			if tc.amministratore {
				if err := service.DB.Exec("UPDATE users SET is_admin = 1 WHERE username = 'prova'").Error; err != nil {
					t.Fatal(err)
				}
			}
			rifiutaLetture(t, "users", strings.Replace(tc.dove, "PROPRIETARIO", idDi(t, "proprietario"), 1))

			rec := invia(tc.metodo, guid.Replace(tc.rotta), "")
			if got, want := rec.Body.String(), `{"error":"Errore di sistema."}`; rec.Code != 400 || got != want {
				t.Errorf("%s %s con gli utenti non letti: %d %s, attesi 400 e %s", tc.metodo, tc.rotta, rec.Code, got, want)
			}
			percorso, _, _ := strings.Cut(tc.rotta, "?")
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// TestAuthQueryUtenteNonLetto prova sul router vero /api/oidc/auth-query, che
// il client interroga finche' il login OIDC non e' fatto, quando l'utente del
// login non si legge: la risposta e' 400 "Errore di sistema.", con l'errore
// nel log, e nessun token. Un utente che non c'e' riceve "Utente non
// trovato.", la risposta che il codice aveva per questo caso ma non dava mai.
// Prima in tutti e due i casi il client riceveva il token di un utente vuoto
// (id 0), e alla prima richiesta un 401, cioe' un logout.
func TestAuthQueryUtenteNonLetto(t *testing.T) {
	for _, tc := range []struct {
		nome, risposta string
		rifiuta        bool
		utente         func(u *model.User) uint
	}{
		{"utente non letto", `{"error":"Errore di sistema."}`, true, func(u *model.User) uint { return u.Id }},
		{"utente che non c'e'", `{"error":"Utente non trovato."}`, false, func(*model.User) uint { return 999999 }},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			preparaLogin(t, utente)
			service.AllService.OauthService.SetOauthCache("login-oidc", &service.OauthCacheItem{UserId: tc.utente(utente), Action: service.OauthActionTypeLogin}, 0)
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("login-oidc") })
			if tc.rifiuta {
				rifiutaLetture(t, "users", "")
			}

			rec := richiesta(g, "GET", "/api/oidc/auth-query?code=login-oidc&id=999000111&uuid=dXVpZA==", "", "")
			if got := rec.Body.String(); rec.Code != 400 || got != tc.risposta {
				t.Errorf("GET /api/oidc/auth-query: %d %s, attesi 400 e %s", rec.Code, got, tc.risposta)
			}
			if n := righe(t, "user_tokens"); n != 1 {
				t.Errorf("token dopo la richiesta: %d, atteso 1, quello del pannello", n)
			}
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, "GET /api/oidc/auth-query: ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("GET /api/oidc/auth-query, nel log mancano rotta o errore:\n%s", nelLog)
			}
		})
	}
}

// TestCallbackOidcLetturaFallita prova sul router vero il callback OIDC
// quando il database non legge l'associazione al provider o un utente: la
// pagina dice OauthFailed, con l'errore nel log, come per gli altri errori
// del callback, e non nascono associazioni. Prima l'associazione chiesta dal
// pannello riusciva lo stesso, anche per un utente che non c'e' (ora
// ItemNotFound, il messaggio che il codice aveva per questo caso ma non dava
// mai), e il login con l'autoregistrazione spenta di un account con la sua
// associazione rimandava ad associarlo di nuovo.
func TestCallbackOidcLetturaFallita(t *testing.T) {
	for _, tc := range []struct {
		nome, azione, messaggio string
		// prepara crea e rifiuta quello che il caso vuole e restituisce
		// l'utente della voce in cache: quello da associare, 0 per un login.
		prepara func(t *testing.T, utente *model.User) uint
	}{
		{"associazione, utente non letto", service.OauthActionTypeBind, "OauthFailed", func(t *testing.T, utente *model.User) uint {
			rifiutaLetture(t, "users", "")
			return utente.Id
		}},
		{"associazione, associazioni non lette", service.OauthActionTypeBind, "OauthFailed", func(t *testing.T, utente *model.User) uint {
			rifiutaLetture(t, "user_thirds", "")
			return utente.Id
		}},
		{"associazione, utente che non c'e'", service.OauthActionTypeBind, "ItemNotFound", func(*testing.T, *model.User) uint {
			return 999999
		}},
		{"login, utente dell'associazione non letto", service.OauthActionTypeLogin, "OauthFailed", func(t *testing.T, utente *model.User) uint {
			crea(t, &model.UserThird{UserId: utente.Id, Op: "aziendale", OauthType: model.OauthTypeOidc, OauthUser: model.OauthUser{OpenId: "sub-1"}})
			rifiutaLetture(t, "users", "")
			return 0
		}},
		{"login, associazione non letta", service.OauthActionTypeLogin, "OauthFailed", func(t *testing.T, utente *model.User) uint {
			crea(t, &model.UserThird{UserId: utente.Id, Op: "aziendale", OauthType: model.OauthTypeOidc, OauthUser: model.OauthUser{OpenId: "sub-1"}})
			rifiutaLetture(t, "user_thirds", "")
			return 0
		}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, registro := pannello(t, false)
			registra := false
			crea(t, &model.Oauth{Op: "aziendale", OauthType: model.OauthTypeOidc, ClientId: "id", ClientSecret: "segreto",
				Issuer: providerOidc(t), AutoRegister: &registra})
			voce := &service.OauthCacheItem{Op: "aziendale", Action: tc.azione, UserId: tc.prepara(t, utente)}
			service.AllService.OauthService.SetOauthCache("prova-callback", voce, 0)
			t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("prova-callback") })
			associazioni, utenti := righe(t, "user_thirds"), righe(t, "users")

			rec := richiesta(g, "GET", "/api/oidc/callback?state=prova-callback&code="+codiceDelProvider, "", "")
			if !strings.Contains(rec.Body.String(), "var msg = '"+tc.messaggio+"'") {
				t.Errorf("GET /api/oidc/callback: %d, atteso il messaggio %s\n%s", rec.Code, tc.messaggio, rec.Body)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, "GET /api/oidc/callback: al client va "+tc.messaggio) {
				t.Errorf("GET /api/oidc/callback, nel log manca la rotta:\n%s", nelLog)
			}
			if n, m := righe(t, "user_thirds"), righe(t, "users"); n != associazioni || m != utenti {
				t.Errorf("dopo il callback %d associazioni e %d utenti, erano %d e %d", n, m, associazioni, utenti)
			}
		})
	}
}

// rifiutaElenchi fa fallire nel database dei servizi le letture senza WHERE
// della tabella tabella, come l'elenco di tutti gli utenti, che
// rifiutaLetture non ferma; le altre riescono.
func rifiutaElenchi(t *testing.T, tabella string) {
	t.Helper()
	err := service.DB.Callback().Query().Before("gorm:query").Register("rifiuta_elenchi_"+tabella, func(db *gorm.DB) {
		if _, ok := db.Statement.Clauses["WHERE"]; db.Statement.Table == tabella && !ok {
			_ = db.AddError(errors.New("lettura rifiutata dal test"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestPannelloUtenteNonLetto prova sul router vero le rotte del pannello che
// leggono un utente o l'elenco degli utenti: se il database non li legge
// rispondono code 101 "Errore di sistema.", con l'errore nel log, e l'utente
// resta com'era. Prima dettaglio, cancellazione e cambio della password
// rispondevano "Elemento non trovato.", la modifica andava in panic (500) e
// gli elenchi erano vuoti.
func TestPannelloUtenteNonLetto(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const modifica = `{"id":SECONDO,"username":"secondo","group_id":1,"status":1,"is_admin":false}`
	for _, tc := range []struct {
		metodo, rotta, corpo, dove string // in rotta, corpo e dove SECONDO diventa l'id di secondo
	}{
		{"GET", "/api/admin/user/detail/SECONDO", "", "id = ? [SECONDO]"},
		{"POST", "/api/admin/user/delete", `{"id":SECONDO}`, "id = ? [SECONDO]"},
		{"POST", "/api/admin/user/changePwd", `{"id":SECONDO,"password":"` + strings.Repeat("p", 15) + `"}`, "id = ? [SECONDO]"},
		{"POST", "/api/admin/user/update", modifica, "id = ? [SECONDO]"},
		{"GET", "/api/admin/user/list?username=sec", "", "username like"},
		{"POST", "/api/admin/user/groupUsers", "", ""},
	} {
		t.Run(tc.metodo+" "+tc.rotta, func(t *testing.T) {
			g, _, registro := pannello(t, true)
			crea(t, &model.User{Username: "secondo", GroupId: 1, Status: model.COMMON_STATUS_ENABLE})
			sostituisci := strings.NewReplacer("SECONDO", idDi(t, "secondo"))
			prima := statoUtenti(t)
			if tc.dove == "" {
				rifiutaElenchi(t, "users")
			} else {
				rifiutaLetture(t, "users", sostituisci.Replace(tc.dove))
			}

			rec := richiesta(g, tc.metodo, sostituisci.Replace(tc.rotta), "", sostituisci.Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != erroreDiSistema {
				t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, erroreDiSistema)
			}
			percorso, _, _ := strings.Cut(strings.Replace(tc.rotta, "/SECONDO", "/:id", 1), "?")
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
			if dopo := statoUtenti(t); dopo != prima {
				t.Errorf("%s %s ha cambiato gli utenti:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
		})
	}
}

// statoUtenti descrive con Raw, che rifiutaLetture non ferma, nome, ruolo,
// stato e password degli utenti.
func statoUtenti(t *testing.T) string {
	t.Helper()
	var utenti []string
	if err := service.DB.Raw("SELECT username || ' ' || is_admin || ' ' || status || ' ' || password FROM users ORDER BY id").Scan(&utenti).Error; err != nil {
		t.Fatal(err)
	}
	return strings.Join(utenti, ", ")
}

// TestPannelloDestinatarioNonLetto prova sul router vero la creazione di una
// regola di condivisione verso un utente: se il database non legge l'utente
// destinatario, amministrazione e sezione dell'utente rispondono code 101
// "Errore di sistema.", con l'errore nel log, e non salvano niente. Prima
// rispondevano "Elemento non trovato.". Un destinatario che non c'e' ha la
// risposta di prima.
func TestPannelloDestinatarioNonLetto(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	const regola = `{"user_id":UTENTE,"collection_id":RUBRICA,"type":1,"rule":2,"to_id":`
	// amico e' il secondo utente di pannelloDiProva, dopo quello di pannello
	provaPannello(t, "users", "id = ? [2]", []casoDelPannello{
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `AMICO}`, true, erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `AMICO}`, true, erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `999999}`, false, nonTrovato},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `999999}`, false, nonTrovato},
	})
}
