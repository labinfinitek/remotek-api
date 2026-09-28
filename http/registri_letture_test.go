package http

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestPannelloRegistriNonLetti prova sul router vero le rotte del pannello
// che leggono l'audit, il registro dei login e i comandi del server. Se il
// database non li legge rispondono code 101 "Errore di sistema.", con
// l'errore nel log, e non cambiano niente. Prima gli elenchi rispondevano
// successo con un elenco vuoto (quello dei comandi con i soli comandi di
// sistema) e le cancellazioni "Elemento non trovato.". Una riga che non
// c'e' ha la risposta di prima.
func TestPannelloRegistriNonLetti(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	for _, tc := range []struct {
		metodo, rotta, corpo string // in corpo CONN, FILE, LOGIN e CMD diventano gli id delle righe
		tabella, dove        string // le letture rifiutate: tutte quelle senza WHERE se dove e' "elenco"
		risposta             string
	}{
		{"GET", "/api/admin/audit_conn/list", "", "audit_conns", "elenco", erroreDiSistema},
		{"POST", "/api/admin/audit_conn/delete", `{"id":CONN}`, "audit_conns", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/audit_file/list", "", "audit_files", "elenco", erroreDiSistema},
		{"POST", "/api/admin/audit_file/delete", `{"id":FILE}`, "audit_files", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/login_log/list", "", "login_logs", "elenco", erroreDiSistema},
		{"POST", "/api/admin/login_log/delete", `{"id":LOGIN}`, "login_logs", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/my/login_log/list", "", "login_logs", "user_id", erroreDiSistema},
		{"POST", "/api/admin/my/login_log/delete", `{"id":LOGIN}`, "login_logs", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/rustdesk/cmdList", "", "server_cmds", "elenco", erroreDiSistema},
		{"POST", "/api/admin/rustdesk/cmdDelete", `{"id":CMD}`, "server_cmds", "id = ?", erroreDiSistema},
		{"POST", "/api/admin/audit_conn/delete", `{"id":999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/audit_file/delete", `{"id":999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/login_log/delete", `{"id":999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/my/login_log/delete", `{"id":999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/rustdesk/cmdDelete", `{"id":999999}`, "", "", nonTrovato},
	} {
		t.Run(tc.metodo+" "+tc.rotta+" "+tc.tabella, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			if err := service.DB.AutoMigrate(&model.AuditConn{}, &model.AuditFile{}, &model.LoginLog{}, &model.ServerCmd{}); err != nil {
				t.Fatal(err)
			}
			conn := &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111"}
			crea(t, conn)
			file := &model.AuditFile{PeerId: "999000111", Path: "C:\\prova"}
			crea(t, file)
			login := &model.LoginLog{UserId: utente.Id, IsDeleted: model.IsDeletedNo}
			crea(t, login)
			cmd := &model.ServerCmd{Cmd: "prova", Target: model.ServerCmdTargetIdServer}
			crea(t, cmd)
			id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
			sostituisci := strings.NewReplacer("CONN", id(conn.Id), "FILE", id(file.Id), "LOGIN", id(login.Id), "CMD", id(cmd.Id))
			prima := statoRegistri(t)
			switch tc.dove {
			case "":
			case "elenco":
				rifiutaElenchi(t, tc.tabella)
			default:
				rifiutaLetture(t, tc.tabella, tc.dove)
			}

			rec := richiesta(g, tc.metodo, tc.rotta, "", sostituisci.Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
				t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
			}
			if tc.dove == "" {
				return
			}
			if dopo := statoRegistri(t); dopo != prima {
				t.Errorf("%s %s ha cambiato il database:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+tc.rotta+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// statoRegistri conta con Raw, che rifiutaLetture non ferma, connessioni e
// file dell'audit, voci del registro dei login non cancellate e comandi del
// server.
func statoRegistri(t *testing.T) string {
	t.Helper()
	var stato []string
	for _, tabella := range []string{"audit_conns", "audit_files", "server_cmds"} {
		stato = append(stato, tabella+" "+strconv.FormatInt(righe(t, tabella), 10))
	}
	var vivi int64
	if err := service.DB.Raw("SELECT count(*) FROM login_logs WHERE is_deleted = ?", model.IsDeletedNo).Scan(&vivi).Error; err != nil {
		t.Fatal(err)
	}
	return strings.Join(append(stato, "login_logs "+strconv.FormatInt(vivi, 10)), ", ")
}
