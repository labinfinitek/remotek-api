package http

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
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
			if err := service.DB.AutoMigrate(&model.Peer{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="}) // l'audit scrive solo dal PC salvato
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

// TestAuditConnessioneNonLetta prova sul router vero la chiusura e la nota di
// una connessione dell'audit quando il database non la legge: la risposta
// resta quella dei golden audit-conn-close e audit-conn-nota, perche' il
// client la ignora, e l'errore va nel log con metodo e rotta. Prima la
// lettura fallita valeva "connessione mai vista" e non restava niente. Una
// connessione che non c'e' resta senza log, come prima.
func TestAuditConnessioneNonLetta(t *testing.T) {
	for _, tc := range []struct {
		nome, corpo string
		rifiuto     bool
	}{
		{"chiusa", `{"action":"close","conn_id":7,"id":"999000111","session_id":1,"uuid":"dXVpZA=="}`, true},
		{"nota", `{"conn_id":7,"id":"999000111","peer":["999000222","tecnico"],"session_id":1,"type":0,"uuid":"dXVpZA=="}`, true},
		{"chiusa, mai vista", `{"action":"close","conn_id":8,"id":"999000111","session_id":1,"uuid":"dXVpZA=="}`, false},
		{"nota, mai vista", `{"conn_id":8,"id":"999000111","peer":["999000222","tecnico"],"session_id":1,"type":0,"uuid":"dXVpZA=="}`, false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			if err := service.DB.AutoMigrate(&model.Peer{}, &model.AuditConn{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="}) // l'audit scrive solo dal PC salvato
			crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111"})
			if tc.rifiuto {
				rifiutaLetture(t, "audit_conns", "peer_id")
			}

			rec := richiesta(g, "POST", "/api/audit/conn", "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
				t.Errorf("POST /api/audit/conn: %d %s, attesi 200 e %s", rec.Code, got, want)
			}
			nelLog := registro.String()
			if tc.rifiuto && (!strings.Contains(nelLog, "POST /api/audit/conn: ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("nel log mancano rotta o errore:\n%s", nelLog)
			}
			if !tc.rifiuto && nelLog != "" {
				t.Errorf("una connessione mai vista non va nel log:\n%s", nelLog)
			}
		})
	}
}

// TestAuditDaAltri prova sul router vero che l'audit di un ID che non e' un
// PC salvato, o con un uuid diverso da quello salvato, non scrive
// niente: nessuna connessione o file nuovo, e la chiusura non chiude la
// connessione del dispositivo salvato. La risposta resta quella dei
// golden audit-conn-new, audit-conn-close e audit-file, il warn nel log ha
// rotta e ID, non gli uuid. Prima scriveva chiunque conoscesse l'ID.
func TestAuditDaAltri(t *testing.T) {
	for _, tc := range []struct {
		nome, rotta, corpo string
		salvato            bool // 999000111 e' un PC salvato con uuidSalvato
	}{
		{"connessione nuova, ID sconosciuto", "/api/audit/conn", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidSalvato + `"}`, false},
		{"connessione nuova, senza uuid", "/api/audit/conn", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":""}`, true},
		{"connessione nuova, uuid diverso", "/api/audit/conn", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidAltro + `"}`, true},
		{"connessione chiusa, uuid diverso", "/api/audit/conn", `{"action":"close","conn_id":7,"id":"999000111","session_id":1,"uuid":"` + uuidAltro + `"}`, true},
		{"connessione autenticata, uuid diverso", "/api/audit/conn", `{"conn_id":7,"id":"999000111","peer":["999000222","intruso"],"session_id":1,"type":0,"uuid":"` + uuidAltro + `"}`, true},
		{"file, ID sconosciuto", "/api/audit/file", `{"conn_id":7,"id":"999000111","info":"{}","is_file":false,"path":"C:\\prova","peer_id":"999000222","type":0,"uuid":"` + uuidSalvato + `"}`, false},
		{"file, uuid diverso", "/api/audit/file", `{"conn_id":7,"id":"999000111","info":"{}","is_file":false,"path":"C:\\prova","peer_id":"999000222","type":0,"uuid":"` + uuidAltro + `"}`, true},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			var pcs []*model.Peer
			if tc.salvato {
				pcs = append(pcs, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
			}
			g, registro := dispositiviDiProva(t, pcs...)
			crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", FromPeer: "999000333"})

			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
				t.Errorf("POST %s: %d %s, attesi 200 e %s", tc.rotta, rec.Code, got, want)
			}
			var conns []model.AuditConn
			if err := service.DB.Raw("SELECT conn_id, peer_id, from_peer, close_time FROM audit_conns").Scan(&conns).Error; err != nil {
				t.Fatal(err)
			}
			want := []model.AuditConn{{ConnId: 7, PeerId: "999000111", FromPeer: "999000333"}}
			if !reflect.DeepEqual(want, conns) {
				t.Errorf("audit_conns: %+v, attese %+v", conns, want)
			}
			if n := righe(t, "audit_files"); n != 0 {
				t.Errorf("audit_files ha %d righe, attese 0", n)
			}
			senzaUuid(t, registro, tc.rotta)
		})
	}
}

// TestAuditPcSenzaUuid prova sul router vero l'audit per un PC creato dal
// pannello senza uuid: non scrive niente, come per un uuid diverso, e il
// warn nel log dice che il PC aspetta il primo sysinfo, non che l'uuid e'
// diverso da quello salvato. Come TestHeartbeatPcSenzaUuid.
func TestAuditPcSenzaUuid(t *testing.T) {
	for _, tc := range []struct{ nome, rotta, corpo string }{
		{"connessione nuova", "/api/audit/conn", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidAltro + `"}`},
		{"file", "/api/audit/file", `{"conn_id":7,"id":"999000111","info":"{}","is_file":false,"path":"C:\\prova","peer_id":"999000222","type":0,"uuid":"` + uuidAltro + `"}`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111"})

			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
				t.Errorf("POST %s: %d %s, attesi 200 e %s", tc.rotta, rec.Code, got, want)
			}
			if n := righe(t, "audit_conns") + righe(t, "audit_files"); n != 0 {
				t.Errorf("audit_conns e audit_files hanno %d righe, attese 0", n)
			}
			senzaUuid(t, registro, tc.rotta)
			if nelLog := registro.String(); !strings.Contains(nelLog, "PC senza uuid, in attesa del primo sysinfo") || strings.Contains(nelLog, "diverso") {
				t.Errorf("POST %s: nel log il motivo non e' quello del PC senza uuid:\n%s", tc.rotta, nelLog)
			}
		})
	}
}

// TestAuditPcNonLetto prova sul router vero che l'audit mandato dal PC
// salvato, col suo uuid, non scrive niente se il database non legge il PC:
// il legame non si puo' controllare. La risposta resta quella dei golden
// audit-conn-new e audit-file, perche' il client la ignora, e l'errore va
// nel log con auditNonSalvato. Prima l'audit non leggeva il PC e scriveva.
func TestAuditPcNonLetto(t *testing.T) {
	for _, tc := range []struct{ nome, rotta, corpo string }{
		{"connessione nuova", "/api/audit/conn", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidSalvato + `"}`},
		{"file", "/api/audit/file", `{"conn_id":7,"id":"999000111","info":"{}","is_file":false,"path":"C:\\prova","peer_id":"999000222","type":0,"uuid":"` + uuidSalvato + `"}`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
			rifiutaLetture(t, "peers", "")

			rec := richiesta(g, "POST", tc.rotta, "", tc.corpo)
			if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
				t.Errorf("POST %s: %d %s, attesi 200 e %s", tc.rotta, rec.Code, got, want)
			}
			for _, tabella := range []string{"audit_conns", "audit_files"} {
				if n := righe(t, tabella); n != 0 {
					t.Errorf("%s ha %d righe, attese 0", tabella, n)
				}
			}
			nelLog := registro.String()
			if logger.Conta(nelLog, "ERROR", "POST "+tc.rotta+": audit non salvato", "lettura rifiutata dal test") != 1 {
				t.Errorf("POST %s, nel log manca l'errore con rotta:\n%s", tc.rotta, nelLog)
			}
		})
	}
}

// TestAuditSessionId prova sul router vero che il session_id, un u64
// casuale nel client, si salva esatto anche sopra 2^53 (golden
// audit-conn-new). Prima passava da float64 e 2^53+1 diventava 2^53.
func TestAuditSessionId(t *testing.T) {
	g, _ := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})

	rec := richiesta(g, "POST", "/api/audit/conn", "", `{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":9007199254740993,"uuid":"`+uuidSalvato+`"}`)
	if rec.Code != 200 {
		t.Fatalf("POST /api/audit/conn: %d %s", rec.Code, rec.Body)
	}
	var ids []string
	if err := service.DB.Raw("SELECT session_id FROM audit_conns").Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	if want := []string{"9007199254740993"}; !reflect.DeepEqual(want, ids) {
		t.Errorf("session_id: %q, atteso %q", ids, want)
	}
}

// TestAuditConnIdRipetuto prova sul router vero che con due "new" dello
// stesso PC e con lo stesso conn_id (il client lo fa ripartire a ogni avvio
// del servizio) la chiusura va sulla connessione piu' recente e la prima
// resta aperta. Prima First prendeva la riga piu' vecchia.
func TestAuditConnIdRipetuto(t *testing.T) {
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	for _, corpo := range []string{
		`{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidSalvato + `"}`,
		`{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.11","session_id":2,"uuid":"` + uuidSalvato + `"}`,
		`{"action":"close","conn_id":7,"id":"999000111","session_id":2,"uuid":"` + uuidSalvato + `"}`,
	} {
		if rec := richiesta(g, "POST", "/api/audit/conn", "", corpo); rec.Code != 200 {
			t.Fatalf("POST /api/audit/conn: %d %s", rec.Code, rec.Body)
		}
	}
	var chiuse []bool
	if err := service.DB.Raw("SELECT close_time > 0 FROM audit_conns ORDER BY id").Scan(&chiuse).Error; err != nil {
		t.Fatal(err)
	}
	if want := []bool{false, true}; !reflect.DeepEqual(want, chiuse) {
		t.Errorf("connessioni chiuse, in ordine di id: %v, attese %v", chiuse, want)
	}
	if nelLog := registro.String(); nelLog != "" {
		t.Errorf("nel log:\n%s", nelLog)
	}
}
