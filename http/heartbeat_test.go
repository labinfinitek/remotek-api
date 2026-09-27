package http

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestHeartbeatNonSalvato prova sul router vero che l'heartbeat del client,
// se il database non aggiorna l'ultimo contatto del dispositivo, risponde
// come sempre 200 {} (golden heartbeat) e scrive l'errore nel log con
// metodo e rotta. Prima si perdeva.
func TestHeartbeatNonSalvato(t *testing.T) {
	g, _, registro := pannello(t, false)
	if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}
	crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="}) // mai visto: l'heartbeat lo aggiorna
	rifiuta(t, "BEFORE UPDATE ON peers")

	rec := richiesta(g, "POST", "/api/heartbeat", "", `{"id":"999000111","modified_at":0,"uuid":"dXVpZA==","ver":1004090}`)
	if rec.Code != 200 || rec.Body.String() != "{}" {
		t.Errorf("POST /api/heartbeat: %d %s, attesi 200 e {}", rec.Code, rec.Body)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "POST /api/heartbeat: ") || !strings.Contains(nelLog, "rifiutato dal test") {
		t.Errorf("POST /api/heartbeat, nel log mancano rotta o errore:\n%s", nelLog)
	}
}
