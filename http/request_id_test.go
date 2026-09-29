package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// TestRequestId prova sul router vero che ogni risposta abbia X-Request-Id,
// diverso da una richiesta all'altra, che un X-Request-Id in arrivo non si
// riprenda, e che la riga di warn scritta dal gestore (qui ParamsError del
// pannello col JSON rotto) abbia come request_id l'id dell'header.
func TestRequestId(t *testing.T) {
	g, _, registro := pannello(t, false)
	var ids []string
	for range 2 {
		req := httptest.NewRequest("POST", "/api/admin/my/tag/create", strings.NewReader(`{"name": x}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api-token", tokenDelPannello)
		req.Header.Set("X-Request-Id", "scelto-dal-chiamante")
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		ids = append(ids, rec.Header().Get("X-Request-Id"))
	}
	if ids[0] == "" || ids[0] == ids[1] {
		t.Fatalf("X-Request-Id %q, attesi due id non vuoti e diversi", ids)
	}
	var righe []logger.Riga
	for _, r := range logger.Righe(registro.String()) {
		if r.Level == "WARN" && strings.Contains(r.Msg, "POST /api/admin/my/tag/create: ") {
			righe = append(righe, r)
		}
	}
	if len(righe) != 2 || righe[0].RequestId != ids[0] || righe[1].RequestId != ids[1] {
		t.Errorf("righe di warn %+v, attese due con request_id %q\n%s", righe, ids, registro)
	}
	for _, id := range ids {
		if id == "scelto-dal-chiamante" {
			t.Error("X-Request-Id del chiamante ripreso come id")
		}
	}
	if strings.Contains(registro.String(), "scelto-dal-chiamante") {
		t.Errorf("X-Request-Id del chiamante nel log:\n%s", registro)
	}
}

// TestRequestIdDelGestore prova sul router vero che anche la riga scritta da
// un gestore del client con global.Logger.Per (qui il warn di /api/heartbeat
// per un uuid diverso da quello salvato) abbia come request_id l'id
// dell'header della sua risposta.
func TestRequestIdDelGestore(t *testing.T) {
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	rec := richiesta(g, "POST", "/api/heartbeat", "", `{"id":"999000111","modified_at":0,"uuid":"`+uuidAltro+`","ver":1004090}`)
	id := rec.Header().Get("X-Request-Id")
	if rec.Code != 200 || id == "" {
		t.Fatalf("POST /api/heartbeat: %d, X-Request-Id %q; attesi 200 e un id", rec.Code, id)
	}
	var righe []logger.Riga
	for _, r := range logger.Righe(registro.String()) {
		if r.Level == "WARN" && strings.Contains(r.Msg, "POST /api/heartbeat: dispositivo 999000111") {
			righe = append(righe, r)
		}
	}
	if len(righe) != 1 || righe[0].RequestId != id {
		t.Errorf("righe di warn %+v, attesa una con request_id %q\n%s", righe, id, registro)
	}
}

// TestRigaDellaRichiesta prova che, a livello debug, il middleware scriva una
// riga per richiesta con request_id, metodo, rotta del router (non l'URL con
// la query), stato e durata, e senza l'IP del chiamante.
func TestRigaDellaRichiesta(t *testing.T) {
	g, _, _ := pannello(t, false)
	var registro strings.Builder
	l, err := logger.NewSu(&registro, &logger.Config{Level: "debug"})
	if err != nil {
		t.Fatal(err)
	}
	global.Logger = l
	req := httptest.NewRequest("POST", "/api/heartbeat?segreto=1", strings.NewReader("{}")) // RemoteAddr 192.0.2.1
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	var riga struct{ Level, RequestId, Method, Route string }
	trovate := 0
	for _, testo := range strings.Split(strings.TrimSpace(registro.String()), "\n") {
		var r map[string]any
		if json.Unmarshal([]byte(testo), &r) != nil || r["msg"] != "richiesta" {
			continue
		}
		trovate++
		riga.Level, _ = r["level"].(string)
		riga.RequestId, _ = r["request_id"].(string)
		riga.Method, _ = r["method"].(string)
		riga.Route, _ = r["route"].(string)
		if _, ok := r["status"].(float64); !ok {
			t.Errorf("riga senza status: %s", testo)
		}
		if _, ok := r["duration_ms"].(float64); !ok {
			t.Errorf("riga senza duration_ms: %s", testo)
		}
	}
	if trovate != 1 || riga.Level != "DEBUG" || riga.RequestId != rec.Header().Get("X-Request-Id") || riga.Method != "POST" || riga.Route != "/api/heartbeat" {
		t.Errorf("%d righe \"richiesta\", l'ultima %+v; attesa una DEBUG con l'id dell'header, POST e /api/heartbeat\n%s", trovate, riga, &registro)
	}
	for _, s := range []string{"segreto", "192.0.2.1"} {
		if strings.Contains(registro.String(), s) {
			t.Errorf("nel log c'e' %q:\n%s", s, &registro)
		}
	}
}
