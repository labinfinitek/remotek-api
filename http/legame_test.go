package http

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// Gli uuid delle prove sul legame ID-uuid (REM-2026-002): quello del
// dispositivo salvato e quello di un altro che ne conosce l'ID.
const (
	uuidSalvato = "dXVpZA=="
	uuidAltro   = "YWx0cm8tdXVpZA=="
)

// dispositiviDiProva prepara il router vero con la tabella peers e i
// dispositivi pcs, e restituisce router e registro del log.
func dispositiviDiProva(t *testing.T, pcs ...*model.Peer) (*gin.Engine, *strings.Builder) {
	t.Helper()
	g, _, registro := pannello(t, false)
	if err := service.DB.AutoMigrate(&model.Peer{}, &model.LoginLog{}, &model.AuditConn{}, &model.AuditFile{}); err != nil {
		t.Fatal(err)
	}
	for _, pc := range pcs {
		crea(t, pc)
	}
	return g, registro
}

// scheda restituisce i campi di 999000111 che un sysinfo o un heartbeat
// possono cambiare, letti con Raw.
func scheda(t *testing.T) []model.Peer {
	t.Helper()
	var pcs []model.Peer
	if err := service.DB.Raw("SELECT id, hostname, username, uuid, last_online_time, last_online_ip FROM peers WHERE id = ?", "999000111").Scan(&pcs).Error; err != nil {
		t.Fatal(err)
	}
	return pcs
}

// senzaUuid controlla che il log abbia un warn con rotta e ID del PC, e
// nessuno dei due uuid.
func senzaUuid(t *testing.T, registro *strings.Builder, rotta string) {
	t.Helper()
	nelLog := registro.String()
	if !strings.Contains(nelLog, "level=warn") || !strings.Contains(nelLog, "POST "+rotta+": ") || !strings.Contains(nelLog, "999000111") {
		t.Errorf("POST %s: nel log manca il warn con rotta e ID:\n%s", rotta, nelLog)
	}
	for _, u := range []string{uuidSalvato, uuidAltro} {
		if strings.Contains(nelLog, u) {
			t.Errorf("POST %s: nel log c'e' l'uuid %s:\n%s", rotta, u, nelLog)
		}
	}
}

func corpoSysinfo(uuid, nome string) string {
	return `{"cpu":"cpu","hostname":"` + nome + `","id":"999000111","memory":"8GB","os":"windows","username":"` + nome + `","uuid":"` + uuid + `","version":"1.4.9"}`
}

// TestSysinfoUuidDiverso prova sul router vero che un sysinfo con l'ID di
// un PC salvato e un altro uuid non cambia la scheda e risponde 400 col
// messaggio DeviceMismatch; nel log il warn ha rotta e ID, non gli uuid.
// Prima la scheda prendeva nome, utente e uuid di chi la mandava.
func TestSysinfoUuidDiverso(t *testing.T) {
	for lingua, messaggio := range map[string]string{
		"it": "Il dispositivo non corrisponde a quello registrato.", //nolint:misspell // testo italiano del messaggio DeviceMismatch
		"en": "The device does not match the registered one.",
	} {
		t.Run(lingua, func(t *testing.T) {
			vero := &model.Peer{Id: "999000111", Hostname: "PC-VERO", Username: "vero", Uuid: uuidSalvato}
			g, registro := dispositiviDiProva(t, vero)
			prima := scheda(t)

			rec := richiesta(g, "POST", "/api/sysinfo", lingua, corpoSysinfo(uuidAltro, "INTRUSO"))
			if want := `{"error":"` + messaggio + `"}`; rec.Code != 400 || rec.Body.String() != want {
				t.Errorf("POST /api/sysinfo: %d %s, attesi 400 e %s", rec.Code, rec.Body, want)
			}
			if got := scheda(t); !reflect.DeepEqual(prima, got) {
				t.Errorf("la scheda e' cambiata: %+v, prima %+v", got, prima)
			}
			senzaUuid(t, registro, "/api/sysinfo")
		})
	}
}

// TestSysinfoPcDalPannello prova sul router vero che un PC creato dal
// pannello senza uuid prende l'uuid del primo sysinfo, e da allora un
// sysinfo con un altro uuid non lo cambia piu'.
func TestSysinfoPcDalPannello(t *testing.T) {
	g, _ := dispositiviDiProva(t, &model.Peer{Id: "999000111", Hostname: "DAL-PANNELLO"})

	if rec := richiesta(g, "POST", "/api/sysinfo", "", corpoSysinfo(uuidSalvato, "PC-VERO")); rec.Code != 200 || rec.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("primo sysinfo: %d %s, attesi 200 e SYSINFO_UPDATED (golden sysinfo)", rec.Code, rec.Body)
	}
	if rec := richiesta(g, "POST", "/api/sysinfo", "", corpoSysinfo(uuidAltro, "INTRUSO")); rec.Code != 400 {
		t.Errorf("sysinfo con un altro uuid: %d %s, atteso 400", rec.Code, rec.Body)
	}
	want := []model.Peer{{Id: "999000111", Hostname: "PC-VERO", Username: "PC-VERO", Uuid: uuidSalvato}}
	if got := scheda(t); !reflect.DeepEqual(want, got) {
		t.Errorf("scheda: %+v, attesa %+v", got, want)
	}
}

// TestSysinfoSenzaUuid prova sul router vero che un sysinfo senza uuid non
// crea il PC e risponde 400: senza uuid non c'e' legame.
func TestSysinfoSenzaUuid(t *testing.T) {
	g, _ := dispositiviDiProva(t)

	if rec := richiesta(g, "POST", "/api/sysinfo", "", corpoSysinfo("", "INTRUSO")); rec.Code != 400 {
		t.Errorf("POST /api/sysinfo: %d %s, atteso 400", rec.Code, rec.Body)
	}
	if n := righe(t, "peers"); n != 0 {
		t.Errorf("peers ha %d righe, attese 0", n)
	}
}

// TestHeartbeatUuidDiverso prova sul router vero che un heartbeat con l'ID
// di un PC salvato e un altro uuid risponde {} (golden heartbeat) e non
// cambia ultimo contatto ne' IP; nel log il warn ha rotta e ID, non gli
// uuid. Prima bastava un uuid qualunque non vuoto.
func TestHeartbeatUuidDiverso(t *testing.T) {
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato, LastOnlineIp: "198.51.100.1"})
	prima := scheda(t)

	rec := richiesta(g, "POST", "/api/heartbeat", "", `{"id":"999000111","modified_at":0,"uuid":"`+uuidAltro+`","ver":1004090}`)
	if rec.Code != 200 || rec.Body.String() != "{}" {
		t.Errorf("POST /api/heartbeat: %d %s, attesi 200 e {}", rec.Code, rec.Body)
	}
	if got := scheda(t); !reflect.DeepEqual(prima, got) {
		t.Errorf("la scheda e' cambiata: %+v, prima %+v", got, prima)
	}
	senzaUuid(t, registro, "/api/heartbeat")
}
