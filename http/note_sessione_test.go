package http

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// testoDellaNota e' il testo delle note di prova: non deve mai finire nel log.
const testoDellaNota = "cliente Rossi, password sul post-it"

// connessioniDiProva prepara il router vero con il PC 999000111 salvato, le
// connessioni conns e un Jwt senza chiave, cosi' RustAuth cerca il token nel
// database; restituisce router e registro del log.
func connessioniDiProva(t *testing.T, conns ...*model.AuditConn) (*gin.Engine, *strings.Builder) {
	t.Helper()
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	precJwt := global.Jwt
	global.Jwt = jwt.NewJwt("", time.Hour)
	t.Cleanup(func() { global.Jwt = precJwt })
	for _, c := range conns {
		crea(t, c)
	}
	return g, registro
}

// daTecnico manda una richiesta del client del tecnico, con Authorization
// Bearer token se non vuoto.
func daTecnico(g *gin.Engine, metodo, rotta, token, corpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, rotta, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// note restituisce conn_id e nota di ogni connessione, in ordine di id.
func note(t *testing.T) map[int64]string {
	t.Helper()
	var conns []model.AuditConn
	if err := service.DB.Raw("SELECT conn_id, note FROM audit_conns ORDER BY id").Scan(&conns).Error; err != nil {
		t.Fatal(err)
	}
	res := map[int64]string{}
	for _, c := range conns {
		res[c.ConnId] = c.Note
	}
	return res
}

// senzaNota controlla che il testo della nota non sia nel log.
func senzaNota(t *testing.T, registro *strings.Builder) {
	t.Helper()
	if nelLog := registro.String(); strings.Contains(nelLog, testoDellaNota) || strings.Contains(nelLog, "Rossi") {
		t.Errorf("nel log c'e' il testo della nota:\n%s", nelLog)
	}
}

// TestNotaDuranteLaSessione prova sul router vero la nota che il PC del
// tecnico manda durante la sessione (golden audit-conn-nota: id, session_id
// e note, senza uuid ne' conn_id): va sulla connessione piu' recente di quel
// PC con quel session_id, non su un'altra sessione dello stesso PC ne' su
// un altro PC; senza connessione non crea righe e scrive una riga di info.
// La risposta resta quella del golden. Prima la nota si perdeva.
func TestNotaDuranteLaSessione(t *testing.T) {
	const ok = `{"code":0,"message":"success","data":""}`
	for _, tc := range []struct {
		nome, sessione string
		want           map[int64]string
		info           bool
	}{
		{"sessione registrata", "9007199254740993", map[int64]string{6: "", 7: testoDellaNota, 8: "", 9: ""}, false},
		{"sessione sconosciuta", "3", map[int64]string{6: "", 7: "", 8: "", 9: ""}, true},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := connessioniDiProva(t,
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 6, PeerId: "999000111", SessionId: "9007199254740993"},
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", SessionId: "9007199254740993"},
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 8, PeerId: "999000111", SessionId: "2"},
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 9, PeerId: "999000222", SessionId: "9007199254740993"},
			)
			rec := daTecnico(g, "POST", "/api/audit/conn", "", `{"id":"999000111","note":"`+testoDellaNota+`","session_id":`+tc.sessione+`}`)
			if rec.Code != 200 || rec.Body.String() != ok {
				t.Errorf("POST /api/audit/conn: %d %s, attesi 200 e %s", rec.Code, rec.Body, ok)
			}
			if got := note(t); !reflect.DeepEqual(tc.want, got) {
				t.Errorf("note per conn_id: %v, attese %v", got, tc.want)
			}
			if nelLog := registro.String(); tc.info != (logger.Conta(nelLog, "INFO", "999000111") == 1) || tc.info != strings.Contains(nelLog, "999000111") {
				t.Errorf("riga di info attesa %v, log:\n%s", tc.info, nelLog)
			}
			senzaNota(t, registro)
		})
	}
}

// TestNotaSessioneZero prova sul router vero che la nota durante la
// sessione con session_id 0, o senza, non scrive niente. Il PC controllato
// col client 1.4.9 registra la connessione nuova con session_id 0 (lr
// ancora vuoto, connection.rs:550 e 1438) e manda quello vero solo dopo
// l'autorizzazione; il client del tecnico non manda mai 0 (client.rs:
// 1866-1871). Prima una nota con solo l'ID del PC, senza token, finiva
// sulla riga di una connessione non autorizzata o in attesa del clic. La
// risposta resta quella del golden, con la riga di info della sessione
// sconosciuta.
func TestNotaSessioneZero(t *testing.T) {
	const ok = `{"code":0,"message":"success","data":""}`
	for _, tc := range []struct{ nome, corpo string }{
		{"session_id 0", `{"id":"999000111","note":"` + testoDellaNota + `","session_id":0}`},
		{"senza session_id", `{"id":"999000111","note":"` + testoDellaNota + `"}`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := connessioniDiProva(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 5, PeerId: "999000111", SessionId: "0", Guid: "in-attesa"})
			rec := daTecnico(g, "POST", "/api/audit/conn", "", tc.corpo)
			if rec.Code != 200 || rec.Body.String() != ok {
				t.Errorf("POST /api/audit/conn: %d %s, attesi 200 e %s", rec.Code, rec.Body, ok)
			}
			if got, want := note(t), map[int64]string{5: ""}; !reflect.DeepEqual(want, got) {
				t.Errorf("note per conn_id: %v, attese %v", got, want)
			}
			if nelLog := registro.String(); logger.Conta(nelLog, "INFO", "nota di sessione senza connessione registrata") != 1 {
				t.Errorf("manca la riga di info, log:\n%s", nelLog)
			}
			senzaNota(t, registro)
		})
	}
}

// TestNotaTroncata prova sul router vero che una nota di 2001 caratteri, per
// tutte e due le strade, si salva coi primi 2000 e che il log lo dice senza
// il testo. Prima non c'era limite.
func TestNotaTroncata(t *testing.T) {
	lunga := strings.Repeat("è", model.NotaMax) + "X"
	for _, tc := range []struct{ nome, metodo, rotta, token, corpo string }{
		{"durante la sessione", "POST", "/api/audit/conn", "", `{"id":"999000111","note":"` + lunga + `","session_id":1}`},
		{"a fine connessione", "PUT", "/api/audit", tokenDelPannello, `{"guid":"abc123","note":"` + lunga + `"}`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := connessioniDiProva(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", SessionId: "1", Guid: "abc123"})
			if rec := daTecnico(g, tc.metodo, tc.rotta, tc.token, tc.corpo); rec.Code != 200 {
				t.Errorf("%s %s: %d %s", tc.metodo, tc.rotta, rec.Code, rec.Body)
			}
			if got, want := note(t)[7], strings.Repeat("è", model.NotaMax); got != want {
				t.Errorf("nota di %d caratteri, attesa di %d", len([]rune(got)), model.NotaMax)
			}
			nelLog := registro.String()
			if !strings.Contains(nelLog, "troncata a 2000") {
				t.Errorf("nel log manca il troncamento:\n%s", nelLog)
			}
			if strings.Contains(nelLog, "èè") {
				t.Errorf("nel log c'e' il testo della nota:\n%s", nelLog)
			}
		})
	}
}

// TestConnessioneAttiva prova sul router vero GET /api/audit/conn/active:
// senza token, o con un token non valido, la risposta di RustAuth; col
// token il guid, come stringa JSON, della connessione aperta di quel PC con
// quel session_id e di quel tipo, e "" se non c'e' (il client riprova) o
// se il session_id e' 0, quello delle connessioni non ancora autorizzate.
// Prima rispondeva 404 (golden audit-conn-active-404).
func TestConnessioneAttiva(t *testing.T) {
	g, _ := connessioniDiProva(t,
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 6, PeerId: "999000111", SessionId: "5", Type: 0, Guid: "aperta"},
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", SessionId: "5", Type: 0, Guid: "chiusa", CloseTime: 1},
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 8, PeerId: "999000111", SessionId: "5", Type: 1, Guid: "file"},
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 9, PeerId: "999000111", SessionId: "6", Type: 0, Guid: "altra-sessione"},
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 10, PeerId: "999000222", SessionId: "5", Type: 0, Guid: "altro-pc"},
		// Come la registra il client 1.4.9 prima dell'autorizzazione.
		&model.AuditConn{Action: model.AuditActionNew, ConnId: 11, PeerId: "999000111", SessionId: "0", Type: 0, Guid: "in-attesa"},
	)
	for _, tc := range []struct {
		nome, query, token string
		stato              int
		want               string
	}{
		{"senza token", "id=999000111&session_id=5&conn_type=0", "", 401, `{"error":"Unauthorized"}`},
		{"token non valido", "id=999000111&session_id=5&conn_type=0", "collaudo-non-valido", 401, `{"error":"Unauthorized"}`},
		{"controllo", "id=999000111&session_id=5&conn_type=0", tokenDelPannello, 200, `"aperta"`},
		{"trasferimento file", "id=999000111&session_id=5&conn_type=1", tokenDelPannello, 200, `"file"`},
		{"sessione senza connessione aperta", "id=999000111&session_id=7&conn_type=0", tokenDelPannello, 200, `""`},
		{"PC senza connessione", "id=999000333&session_id=5&conn_type=0", tokenDelPannello, 200, `""`},
		{"session_id 0", "id=999000111&session_id=0&conn_type=0", tokenDelPannello, 200, `""`},
	} {
		rec := daTecnico(g, "GET", "/api/audit/conn/active?"+tc.query, tc.token, "")
		if rec.Code != tc.stato || rec.Body.String() != tc.want {
			t.Errorf("%s: %d %s, attesi %d e %s", tc.nome, rec.Code, rec.Body, tc.stato, tc.want)
		}
	}
}

// TestNotaFineConnessione prova sul router vero PUT /api/audit: col token
// la nota va sulla connessione con quel guid; un guid sconosciuto, o vuoto
// come quello delle connessioni di prima, risponde 400 ItemNotFound e non
// scrive; senza token la risposta di RustAuth. Prima rispondeva 404
// (golden audit-nota-guid-404).
func TestNotaFineConnessione(t *testing.T) {
	for _, tc := range []struct {
		nome, token, guid string
		stato             int
		risposta          string
		want              map[int64]string
	}{
		{"guid giusto", tokenDelPannello, "abc123", 200, `{"code":0,"message":"success","data":""}`, map[int64]string{6: "", 7: testoDellaNota}},
		{"guid sconosciuto", tokenDelPannello, "collaudo", 400, `{"error":"Elemento non trovato."}`, map[int64]string{6: "", 7: ""}},
		{"guid vuoto", tokenDelPannello, "", 400, `{"error":"Elemento non trovato."}`, map[int64]string{6: "", 7: ""}},
		{"senza token", "", "abc123", 401, `{"error":"Unauthorized"}`, map[int64]string{6: "", 7: ""}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := connessioniDiProva(t,
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 6, PeerId: "999000111", SessionId: "1"},
				&model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", SessionId: "1", Guid: "abc123"},
			)
			rec := daTecnico(g, "PUT", "/api/audit", tc.token, `{"guid":"`+tc.guid+`","note":"`+testoDellaNota+`"}`)
			if rec.Code != tc.stato || rec.Body.String() != tc.risposta {
				t.Errorf("PUT /api/audit: %d %s, attesi %d e %s", rec.Code, rec.Body, tc.stato, tc.risposta)
			}
			if got := note(t); !reflect.DeepEqual(tc.want, got) {
				t.Errorf("note per conn_id: %v, attese %v", got, tc.want)
			}
			senzaNota(t, registro)
		})
	}
}

// TestGuidDellaConnessione prova sul router vero che la connessione nuova
// (golden audit-conn-new) nasce con un guid casuale di 128 bit in
// esadecimale, diverso per ogni connessione, e che il JSON del pannello
// mostra la nota ma non il guid. Prima il guid non c'era.
func TestGuidDellaConnessione(t *testing.T) {
	g, _ := connessioniDiProva(t)
	for _, conn := range []string{"7", "8"} {
		rec := daTecnico(g, "POST", "/api/audit/conn", "", `{"action":"new","conn_id":`+conn+`,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"`+uuidSalvato+`"}`)
		if rec.Code != 200 {
			t.Fatalf("POST /api/audit/conn: %d %s", rec.Code, rec.Body)
		}
	}
	var guid []string
	if err := service.DB.Raw("SELECT guid FROM audit_conns ORDER BY id").Scan(&guid).Error; err != nil {
		t.Fatal(err)
	}
	esadecimale := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if len(guid) != 2 || !esadecimale.MatchString(guid[0]) || !esadecimale.MatchString(guid[1]) || guid[0] == guid[1] {
		t.Errorf("guid: %q, attesi due diversi di 32 cifre esadecimali", guid)
	}
	b, err := json.Marshal(&model.AuditConn{Note: "nota", Guid: "segreto"})
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"note":"nota"`) || strings.Contains(s, "segreto") || strings.Contains(s, "guid") {
		t.Errorf("JSON del pannello: %s", s)
	}
}
