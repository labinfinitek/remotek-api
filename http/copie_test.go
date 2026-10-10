package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestPannelloPossibileCopia prova sul router vero la rilevazione delle
// copie (ADR-0024): un PC i cui heartbeat arrivano alternati da due
// indirizzi ha il segno della copia vero nella lista del pannello, con un WARN
// nel log; un PC con un solo indirizzo ce l'ha falso, anche se chi ne
// conosce l'ID manda heartbeat con un altro uuid da altri indirizzi. L'heartbeat risponde
// sempre 200 {} (golden heartbeat).
func TestPannelloPossibileCopia(t *testing.T) {
	g, _, registro := pannello(t, true)
	if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}
	// ID che nessun altro test usa: la memoria delle copie e' del processo.
	crea(t, &model.Peer{Id: "999000611", Uuid: "Y29waWE="})
	crea(t, &model.Peer{Id: "999000612", Uuid: "dmVybw=="})
	heartbeat := func(id, uuid, indirizzo string) {
		req := httptest.NewRequest("POST", "/api/heartbeat", strings.NewReader(`{"id":"`+id+`","modified_at":0,"uuid":"`+uuid+`","ver":1004090}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = indirizzo + ":40000"
		rec := httptest.NewRecorder()
		g.ServeHTTP(rec, req)
		if rec.Code != 200 || rec.Body.String() != "{}" {
			t.Fatalf("POST /api/heartbeat: %d %s, attesi 200 e {}", rec.Code, rec.Body)
		}
	}
	for i := range service.SogliaCopie + 1 {
		heartbeat("999000611", "Y29waWE=", []string{"198.51.100.1", "203.0.113.7"}[i%2])
		heartbeat("999000612", "dmVybw==", "198.51.100.2")
		// ID sconosciuto e uuid diverso non entrano nella memoria.
		heartbeat("999000613", "Y29waWE=", []string{"198.51.100.1", "203.0.113.7"}[i%2])
		heartbeat("999000612", "YWx0cm8=", []string{"198.51.100.3", "203.0.113.9"}[i%2])
	}
	if service.PossibileCopia("999000613", time.Now()) {
		t.Error("ID sconosciuto: copia sospetta")
	}

	rec := richiesta(g, "GET", "/api/admin/peer/list?page=1&page_size=10", "", "")
	var risposta struct {
		Code int
		Data struct {
			List []struct {
				Id             string `json:"id"`
				PossibileCopia *bool  `json:"possibile_copia"` //nolint:misspell // italiano, nome deciso in ADR-0024
			}
			Total int64
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &risposta); err != nil || rec.Code != 200 || risposta.Code != 0 {
		t.Fatalf("GET /api/admin/peer/list: %d %s (%v)", rec.Code, rec.Body, err)
	}
	attesi := map[string]bool{"999000611": true, "999000612": false}
	if risposta.Data.Total != 2 || len(risposta.Data.List) != 2 {
		t.Fatalf("GET /api/admin/peer/list: %s, attesi 2 PC", rec.Body)
	}
	for _, p := range risposta.Data.List {
		if p.PossibileCopia == nil || *p.PossibileCopia != attesi[p.Id] {
			t.Errorf("PC %s: segno della copia %v, atteso %v\n%s", p.Id, p.PossibileCopia, attesi[p.Id], rec.Body)
		}
	}
	log := registro.String()
	if strings.Count(log, service.MessaggioCopia) != 1 || !strings.Contains(log, `"id":"999000611"`) {
		t.Errorf("nel log serve un WARN per 999000611:\n%s", log)
	}
	if strings.Contains(log, "Y29waWE=") {
		t.Errorf("uuid nel log:\n%s", log)
	}
}
