package http

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestAuditNonSalvato prova sul router vero le richieste dell'audit del
// client quando il database non salva: la risposta resta quella di sempre,
// {"code":0,"message":"success","data":""} (golden audit-conn-new,
// audit-conn-close, audit-conn-auth, audit-file), perche' il client la
// ignora, e l'errore va nel log con metodo e rotta. Prima si perdeva.
func TestAuditNonSalvato(t *testing.T) {
	for _, tc := range []struct {
		nome, rotta, corpo, ostacolo string
		esistente                    bool // la connessione 7 del dispositivo e' gia' nell'audit
	}{
		{"connessione nuova", "/api/audit/conn", `{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"dXVpZA=="}`,
			"BEFORE INSERT ON audit_conns", false},
		{"connessione chiusa", "/api/audit/conn", `{"action":"close","conn_id":7,"id":"999000111","session_id":1,"uuid":"dXVpZA=="}`,
			"BEFORE UPDATE ON audit_conns", true},
		{"connessione autenticata", "/api/audit/conn", `{"conn_id":7,"id":"999000111","peer":["999000222","tecnico"],"session_id":1,"type":0,"uuid":"dXVpZA=="}`,
			"BEFORE UPDATE ON audit_conns", true},
		{"file", "/api/audit/file", `{"conn_id":7,"id":"999000111","info":"{}","is_file":false,"path":"C:\\prova","peer_id":"999000222","type":0,"uuid":"dXVpZA=="}`,
			"BEFORE INSERT ON audit_files", false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			if err := service.DB.AutoMigrate(&model.AuditConn{}, &model.AuditFile{}); err != nil {
				t.Fatal(err)
			}
			if tc.esistente {
				crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111"})
			}
			rifiuta(t, tc.ostacolo)

			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
				t.Errorf("POST %s: %d %s, attesi 200 e %s", tc.rotta, rec.Code, got, want)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, "POST "+tc.rotta+": ") || !strings.Contains(nelLog, "rifiutato dal test") {
				t.Errorf("POST %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}
		})
	}
}
