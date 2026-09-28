package http

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// uuidDiProva e' l'uuid del PC delle prove: autorizza a scriverne scheda,
// heartbeat e audit (REM-2026-002), e va solo agli amministratori.
const uuidDiProva = "dXVpZC1kZWwtcGMtZGktcHJvdmE="

// TestUuidAlPannello prova sul router vero che il pannello manda l'uuid dei
// dispositivi solo agli amministratori. Un utente non amministratore vede nei
// suoi dispositivi (/api/admin/my/peer/list) e nei suoi login
// (/api/admin/my/login_log/list) le righe sue, ma senza l'uuid salvato;
// l'amministratore lo vede ancora in /api/admin/peer/list e
// /api/admin/login_log/list. Prima le due liste di admin/my davano l'uuid a
// chiunque avesse un PC legato al proprio utente.
func TestUuidAlPannello(t *testing.T) {
	for _, tc := range []struct {
		nome    string
		admin   bool
		rotte   []string
		conUuid bool
	}{
		{"utente", false, []string{"/api/admin/my/peer/list", "/api/admin/my/login_log/list"}, false},
		{"amministratore", true, []string{"/api/admin/peer/list", "/api/admin/login_log/list"}, true},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, utente, _ := pannello(t, tc.admin)
			if err := service.DB.AutoMigrate(&model.Peer{}, &model.LoginLog{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000111", Uuid: uuidDiProva, UserId: utente.Id, Hostname: "PC-COLLAUDO"})
			crea(t, &model.LoginLog{UserId: utente.Id, Client: model.LoginLogClientApp, DeviceId: "999000111", Uuid: uuidDiProva})

			for _, rotta := range tc.rotte {
				rec := richiesta(g, "GET", rotta+"?page=1&page_size=10", "", "")
				corpo := rec.Body.String()
				if rec.Code != 200 || !strings.Contains(corpo, `"code":0`) || !strings.Contains(corpo, "999000111") {
					t.Fatalf("GET %s (admin %v): %d %s, attesi 200, code 0 e la riga di 999000111", rotta, tc.admin, rec.Code, corpo)
				}
				if got := strings.Contains(corpo, uuidDiProva); got != tc.conUuid {
					t.Errorf("GET %s (admin %v): uuid nella risposta %v, atteso %v:\n%s", rotta, tc.admin, got, tc.conUuid, corpo)
				}
			}
		})
	}
}
