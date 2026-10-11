package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// accendiIntegrazione crea la tabella della coda e imposta
// integrazione.url se url non e' vuoto.
func accendiIntegrazione(t *testing.T, url string) {
	t.Helper()
	if err := service.DB.AutoMigrate(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	prec := global.Config.Integrazione
	t.Cleanup(func() { global.Config.Integrazione = prec })
	global.Config.Integrazione = config.Integrazione{Url: url, Segreto: strings.Repeat("s", 32)}
}

// corpiInCoda restituisce i corpi degli eventi in coda, in ordine, decodificati.
func corpiInCoda(t *testing.T) []map[string]any {
	t.Helper()
	var coda []model.Evento
	if err := service.DB.Order("id").Find(&coda).Error; err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, e := range coda {
		var c map[string]any
		if err := json.Unmarshal([]byte(e.Corpo), &c); err != nil {
			t.Fatal(err)
		}
		c["corpo"] = e.Corpo
		res = append(res, c)
	}
	return res
}

const webhook = "https://n8n.example/webhook/remotek"

// trasferimenti manda a /api/audit/file due trasferimenti, come nel golden
// audit-file: type 0 di un file e type 1 di tre.
func trasferimenti(t *testing.T, g *gin.Engine) {
	t.Helper()
	for _, corpo := range []string{
		`{"conn_id":7,"id":"999000111","info":"{\"files\":[[\"prova.txt\",12]],\"ip\":\"192.0.2.10\",\"name\":\"tecnico\",\"num\":1}","is_file":false,"path":"C:\\Users\\collaudo\\Desktop","peer_id":"999000222","type":0,"uuid":"` + uuidSalvato + `"}`,
		`{"conn_id":7,"id":"999000111","info":"{\"files\":[[\"a.txt\",1]],\"ip\":\"192.0.2.10\",\"name\":\"tecnico\",\"num\":3}","is_file":true,"path":"C:\\Users\\collaudo\\a.txt","peer_id":"999000222","type":1,"uuid":"` + uuidSalvato + `"}`,
	} {
		if rec := daTecnico(g, "POST", "/api/audit/file", "", corpo); rec.Code != 200 || rec.Body.String() != `{"code":0,"message":"success","data":""}` {
			t.Errorf("POST /api/audit/file: %d %s", rec.Code, rec.Body)
		}
	}
}

// TestEventoFileTrasferiti: ogni trasferimento salvato accoda
// file.trasferiti con direzione e numero, senza nomi, percorsi o IP.
func TestEventoFileTrasferiti(t *testing.T) {
	g, _ := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	accendiIntegrazione(t, webhook)
	trasferimenti(t, g)
	coda := corpiInCoda(t)
	if len(coda) != 2 {
		t.Fatalf("%d eventi, attesi 2", len(coda))
	}
	for i, want := range []map[string]any{{"direzione": "dal_pc", "numero": 1.0}, {"direzione": "al_pc", "numero": 3.0}} {
		c := coda[i]
		if c["tipo"] != service.EventoFileTrasferiti || !jsonUguale(c["file"], want) {
			t.Errorf("evento %d: %v, atteso file %v", i, c, want)
		}
		for _, vietato := range []string{"prova.txt", "a.txt", "Users", "192.0.2.10", "999000222"} {
			if strings.Contains(c["corpo"].(string), vietato) {
				t.Errorf("nel corpo c'e' %s: %s", vietato, c["corpo"])
			}
		}
	}
}

// jsonUguale confronta due valori decodificati da JSON.
func jsonUguale(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

// TestEventoTerminaleChiuso: i blocchi senza fine non accodano niente;
// quello con fine accoda terminale.chiuso con l'esito della verifica del
// pannello, integra e con lo stesso hash finale, senza blocchi ne' dati.
func TestEventoTerminaleChiuso(t *testing.T) {
	g, _ := terminaleDiProva(t, true, model.AuditConnTerminale)
	accendiIntegrazione(t, webhook)
	bb := catena("whoami\n", "rossi\\mario\n", "exit\n")
	manda(t, g, bb[:2]...)
	if n := len(corpiInCoda(t)); n != 0 {
		t.Fatalf("%d eventi prima della fine, attesi 0", n)
	}
	manda(t, g, bb[2])
	coda := corpiInCoda(t)
	if len(coda) != 1 {
		t.Fatalf("%d eventi, atteso 1", len(coda))
	}
	v := verifica(t, g, 1)
	c := coda[0]
	want := map[string]any{"integra": true, "hash": v.Hash}
	if c["tipo"] != service.EventoTerminaleChiuso || !jsonUguale(c["verifica"], want) || v.Hash != bb[2].Hash {
		t.Errorf("evento %v, attesa verifica %v (hash dell'ultimo blocco %s)", c, want, bb[2].Hash)
	}
	if s, ok := c["sessione"].(map[string]any); !ok || s["tipo"] != 4.0 || s["conn_id"] != 7.0 {
		t.Errorf("sessione dell'evento: %v", c["sessione"])
	}
	for _, b := range bb {
		if strings.Contains(c["corpo"].(string), b.Data) {
			t.Errorf("nel corpo ci sono i dati di un blocco: %s", c["corpo"])
		}
	}
	for _, vietato := range []string{"whoami", "rossi", "blocchi", bb[0].Hash} {
		if strings.Contains(c["corpo"].(string), vietato) {
			t.Errorf("nel corpo c'e' %s: %s", vietato, c["corpo"])
		}
	}
}

// TestEventoPcNuovo: il sysinfo che crea il PC accoda pc.nuovo con ID,
// hostname e alias; il sysinfo dopo, sullo stesso PC, no.
func TestEventoPcNuovo(t *testing.T) {
	g, _ := dispositiviDiProva(t)
	accendiIntegrazione(t, webhook)
	for range 2 {
		if rec := richiesta(g, "POST", "/api/sysinfo", "", corpoSysinfo(uuidSalvato, "PC-ROSSI")); rec.Code != 200 || rec.Body.String() != "SYSINFO_UPDATED" {
			t.Fatalf("POST /api/sysinfo: %d %s", rec.Code, rec.Body)
		}
	}
	coda := corpiInCoda(t)
	want := map[string]any{"id": "999000111", "nome": "PC-ROSSI", "alias": ""}
	if len(coda) != 1 || coda[0]["tipo"] != service.EventoPcNuovo || !jsonUguale(coda[0]["pc"], want) {
		t.Fatalf("coda %v, atteso un pc.nuovo con pc %v", coda, want)
	}
	if strings.Contains(coda[0]["corpo"].(string), uuidSalvato) {
		t.Errorf("uuid nel corpo: %s", coda[0]["corpo"])
	}
}

// heartbeatDa manda l'heartbeat del PC id dall'indirizzo indirizzo.
func heartbeatDa(t *testing.T, g *gin.Engine, id, uuid, indirizzo string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/heartbeat", strings.NewReader(`{"id":"`+id+`","modified_at":0,"uuid":"`+uuid+`","ver":1004090}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = indirizzo + ":40000"
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "{}" {
		t.Fatalf("POST /api/heartbeat: %d %s, attesi 200 e {}", rec.Code, rec.Body)
	}
}

// TestEventoPossibileCopia: col warn della copia esce pc.possibile_copia,
// uno solo come il warn, col numero di cambi e senza gli indirizzi.
func TestEventoPossibileCopia(t *testing.T) {
	// ID che nessun altro test usa: la memoria delle copie e' del processo.
	g, _ := dispositiviDiProva(t, &model.Peer{Id: "999000621", Uuid: "Y29waWE=", Hostname: "PC-COPIA"})
	accendiIntegrazione(t, webhook)
	for i := range service.SogliaCopie + 3 {
		heartbeatDa(t, g, "999000621", "Y29waWE=", []string{"198.51.100.1", "203.0.113.7"}[i%2])
	}
	coda := corpiInCoda(t)
	if len(coda) != 1 || coda[0]["tipo"] != service.EventoPcCopia || !jsonUguale(coda[0]["copia"], map[string]any{"cambi": service.SogliaCopie}) {
		t.Fatalf("coda %v, atteso un pc.possibile_copia con %d cambi", coda, service.SogliaCopie)
	}
	for _, vietato := range []string{"198.51.100.1", "203.0.113.7", "Y29waWE="} {
		if strings.Contains(coda[0]["corpo"].(string), vietato) {
			t.Errorf("nel corpo c'e' %s: %s", vietato, coda[0]["corpo"])
		}
	}
}

// TestAltriEventiSenzaUrl: senza integrazione.url trasferimenti, fine di
// una trascrizione, PC nuovo e copia non accodano niente.
func TestAltriEventiSenzaUrl(t *testing.T) {
	g, _ := terminaleDiProva(t, true, model.AuditConnTerminale)
	accendiIntegrazione(t, "")
	if err := service.DB.AutoMigrate(&model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	trasferimenti(t, g)
	manda(t, g, catena("exit\n")...)
	if rec := richiesta(g, "POST", "/api/sysinfo", "", strings.Replace(corpoSysinfo(uuidAltro, "NUOVO"), "999000111", "999000622", 1)); rec.Code != 200 {
		t.Fatalf("POST /api/sysinfo: %d %s", rec.Code, rec.Body)
	}
	for i := range service.SogliaCopie + 1 {
		heartbeatDa(t, g, "999000622", uuidAltro, []string{"198.51.100.1", "203.0.113.7"}[i%2])
	}
	if coda := corpiInCoda(t); len(coda) != 0 {
		t.Errorf("%d eventi in coda, attesi 0: %v", len(coda), coda)
	}
}
