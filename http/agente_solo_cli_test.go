package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestAgenteSoloCli prova sul router vero che un agente AI usa solo le rotte
// del client CLI (ADR-0028): le sue rispondono come sempre,
// le altre con login (rubriche, PC, utenti) 403 col messaggio e un warn,
// mentre per una persona non cambia niente.
func TestAgenteSoloCli(t *testing.T) {
	g, _, registro := lasciapassareDiProva(t, true)
	const rifiuto = `{"error":"Un account di agente AI usa solo il client CLI."}`
	for _, tc := range []struct {
		metodo, rotta, token, corpo string
		vietata                     bool
	}{
		{"GET", "/api/agente", "tok-agente-mario", "", false},
		{"POST", "/api/currentUser", "tok-agente-mario", "", false},
		{"GET", "/api/user/info", "tok-agente-mario", "", false},
		{"POST", "/api/agente/lasciapassare", "tok-agente-mario", `{"id":"999000111"}`, false},
		{"GET", "/api/ab", "tok-agente-mario", "", true},
		{"POST", "/api/ab", "tok-agente-mario", `{"data":"{}"}`, true},
		{"POST", "/api/ab/personal", "tok-agente-mario", "", true},
		{"POST", "/api/ab/peer/add/abc", "tok-agente-mario", `{"id":"999000999"}`, true},
		{"GET", "/api/peers", "tok-agente-mario", "", true},
		{"GET", "/api/users", "tok-agente-mario", "", true},
		{"GET", "/api/audit/conn/active", "tok-agente-mario", "", true},
		{"GET", "/api/ab", "tok-mario", "", false},
		{"POST", "/api/ab/personal", "tok-mario", "", false},
	} {
		rec := daTecnico(g, tc.metodo, tc.rotta, tc.token, tc.corpo)
		if got := rec.Code == 403 && rec.Body.String() == rifiuto; got != tc.vietata {
			t.Errorf("%s %s col token %s: %d %s, vietata attesa %t", tc.metodo, tc.rotta, tc.token, rec.Code, rec.Body, tc.vietata)
		}
	}
	if n := applog.Conta(registro.String(), "WARN", "rifiutato l'agente AI 4"); n != 7 {
		t.Errorf("%d warn sull'agente, attesi 7:\n%s", n, registro)
	}
	// Il logout resta, ed e' l'ultima: chiude la sessione dell'agente.
	if rec := daTecnico(g, "POST", "/api/logout", "tok-agente-mario", ""); rec.Code == 403 {
		t.Errorf("POST /api/logout dell'agente: %d %s", rec.Code, rec.Body)
	}
}

// TestAgenteSenzaPannello prova sul router vero che un agente AI non entra
// nel pannello: l'aggiramento del confine provato sulla #132 (rubrica sua,
// con un PC qualunque, condivisa col tecnico) si ferma alla prima rotta,
// con code 403 e senza scrivere niente; per il tecnico la rotta funziona.
func TestAgenteSenzaPannello(t *testing.T) {
	g, _, _ := lasciapassareDiProva(t, true)
	pannelloDi := func(token, rotta, corpo string) string {
		req := httptest.NewRequest("POST", rotta, strings.NewReader(corpo))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api-token", token)
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		return rec.Body.String()
	}
	for _, rotta := range []string{"/api/admin/my/address_book_collection/create", "/api/admin/my/address_book_collection_rule/create", "/api/admin/my/address_book/create"} {
		if body := pannelloDi("tok-agente-mario", rotta, `{"name":"mia","id":"999000999","collection_id":1,"rule":1,"type":1,"to_id":2}`); !strings.Contains(body, `"code":403`) {
			t.Errorf("%s dall'agente: %s, atteso code 403", rotta, body)
		}
	}
	var n int64
	if err := service.DB.Model(&model.AddressBookCollection{}).Count(&n).Error; err != nil || n != 0 {
		t.Errorf("%d rubriche create (%v), attese 0", n, err)
	}
	if body := pannelloDi("tok-mario", "/api/admin/my/address_book_collection/create", `{"name":"clienti"}`); !strings.Contains(body, `"code":0`) {
		t.Errorf("rubrica del tecnico: %s, atteso code 0", body)
	}
}

// TestAgenteLoginPannello prova sul router vero che un agente AI, con la
// password giusta o col login OIDC, non ottiene un token del pannello (code
// 101 col messaggio, nessun token che resti), mentre il tecnico entra come
// sempre.
func TestAgenteLoginPannello(t *testing.T) {
	g, _, _ := lasciapassareDiProva(t, true)
	for _, id := range []uint{2, 4} {
		u, err := service.AllService.UserService.InfoById(id)
		if err != nil {
			t.Fatal(err)
		}
		preparaLogin(t, u)
	}
	const pwd = passwordDiProva
	token := func() int64 {
		var n int64
		if err := service.DB.Model(&model.UserToken{}).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	prima := token()
	rec := daTecnico(g, "POST", "/api/admin/login", "", `{"username":"agente-mario","password":"`+pwd+`"}`)
	if body := rec.Body.String(); !strings.Contains(body, `"code":101`) || !strings.Contains(body, "Un account di agente AI usa solo il client CLI.") || strings.Contains(body, `"token"`) || token() != prima {
		t.Errorf("login dell'agente: %s, token %d -> %d", body, prima, token())
	}
	// Login OIDC del pannello: OidcAuthQueryPre crea il token, che si revoca.
	service.AllService.OauthService.SetOauthCache("oidc-agente", &service.OauthCacheItem{UserId: 4, Action: service.OauthActionTypeLogin}, 0)
	t.Cleanup(func() { service.AllService.OauthService.DeleteOauthCache("oidc-agente") })
	rec = daTecnico(g, "GET", "/api/admin/oidc/auth-query?code=oidc-agente", "", "")
	if body := rec.Body.String(); !strings.Contains(body, `"code":101`) || !strings.Contains(body, "Un account di agente AI usa solo il client CLI.") || strings.Contains(body, `"token"`) || token() != prima {
		t.Errorf("login OIDC dell'agente: %s, token %d -> %d", body, prima, token())
	}
	if rec := daTecnico(g, "POST", "/api/admin/login", "", `{"username":"mario","password":"`+pwd+`"}`); !strings.Contains(rec.Body.String(), `"code":0`) || !strings.Contains(rec.Body.String(), `"token"`) {
		t.Errorf("login del tecnico: %d %s", rec.Code, rec.Body)
	}
}
