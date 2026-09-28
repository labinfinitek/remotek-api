package http

// Test delle letture dei dispositivi e dei gruppi sul router vero: un errore
// del database non vale "dispositivo nuovo", "gruppo non condiviso" o un
// elenco vuoto.

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestSysinfoDispositivoNonLetto prova sul router vero /api/sysinfo, senza
// autenticazione. Se il database non legge il dispositivo risponde 400
// {"error": "Errore di sistema."}, con l'errore nel log, e non crea niente: il
// client 1.4.9 riprova piu' tardi. Prima lo prendeva per nuovo e ne creava un
// altro con lo stesso id, un doppione (peers.id non e' unico). Senza ostacoli
// il dispositivo che c'e' si aggiorna e quello che non c'e' si crea, con
// SYSINFO_UPDATED (golden sysinfo).
func TestSysinfoDispositivoNonLetto(t *testing.T) {
	for _, tc := range []struct {
		nome, id  string
		rifiuta   bool
		stato     int
		corpo     string
		nuoviPeer int64
	}{
		{"dispositivo non letto", "999000111", true, 400, `{"error":"Errore di sistema."}`, 0},
		{"dispositivo che c'e'", "999000111", false, 200, "SYSINFO_UPDATED", 0},
		{"dispositivo nuovo", "999000222", false, 200, "SYSINFO_UPDATED", 1},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _, registro := pannello(t, false)
			if err := service.DB.AutoMigrate(&model.LoginLog{}, &model.Peer{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="})
			prima := righe(t, "peers")
			if tc.rifiuta {
				rifiutaLetture(t, "peers", "")
			}

			rec := richiesta(g, "POST", "/api/sysinfo", "", `{"id":"`+tc.id+`","uuid":"dXVpZA==","hostname":"PC-COLLAUDO","os":"windows","version":"1.4.9"}`)
			if rec.Code != tc.stato || rec.Body.String() != tc.corpo {
				t.Errorf("POST /api/sysinfo: %d %s, attesi %d e %s", rec.Code, rec.Body, tc.stato, tc.corpo)
			}
			if dopo := righe(t, "peers"); dopo != prima+tc.nuoviPeer {
				t.Errorf("dopo /api/sysinfo %d dispositivi, erano %d: attesi %d nuovi", dopo, prima, tc.nuoviPeer)
			}
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, "POST /api/sysinfo: ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("POST /api/sysinfo, nel log mancano rotta o errore:\n%s", nelLog)
			}
		})
	}
}

// TestHeartbeatDispositivoNonLetto prova sul router vero che l'heartbeat,
// se il database non legge il dispositivo, risponde come sempre 200 {}
// (golden heartbeat) e scrive l'errore nel log con metodo e rotta. Prima
// l'ultimo contatto non si aggiornava, senza traccia.
func TestHeartbeatDispositivoNonLetto(t *testing.T) {
	g, _, registro := pannello(t, false)
	if err := service.DB.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatal(err)
	}
	crea(t, &model.Peer{Id: "999000111", Uuid: "dXVpZA=="})
	rifiutaLetture(t, "peers", "")

	rec := richiesta(g, "POST", "/api/heartbeat", "", `{"id":"999000111","modified_at":0,"uuid":"dXVpZA==","ver":1004090}`)
	if rec.Code != 200 || rec.Body.String() != "{}" {
		t.Errorf("POST /api/heartbeat: %d %s, attesi 200 e {}", rec.Code, rec.Body)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "POST /api/heartbeat: ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
		t.Errorf("POST /api/heartbeat, nel log mancano rotta o errore:\n%s", nelLog)
	}
}

// TestAggiuntaVoceDispositivoNonLetto prova sul router vero l'aggiunta di
// una voce senza piattaforma, utente e nome del computer, che li prende dal
// dispositivo. Se il database non legge il dispositivo risponde 400
// {"error": "Errore di sistema."}, con l'errore nel log, e la voce non nasce,
// come in POST /api/ab. Prima nasceva senza quei campi. Senza ostacoli la
// voce nasce coi dati del dispositivo, e senza dispositivo nasce senza.
func TestAggiuntaVoceDispositivoNonLetto(t *testing.T) {
	for _, tc := range []struct {
		nome, id string
		rifiuta  bool
		stato    int
		corpo    string
		voce     string // hostname della voce nuova; "-" se non nasce
	}{
		{"dispositivo non letto", "999000333", true, 400, `{"error":"Errore di sistema."}`, "-"},
		{"dispositivo che c'e'", "999000333", false, 200, "", "PC-COLLAUDO"},
		{"dispositivo che non c'e'", "999000444", false, 200, "", ""},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, registro, invia, guid := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			crea(t, &model.Peer{Id: "999000333", Os: "windows", Username: "utente", Hostname: "PC-COLLAUDO"})
			if tc.rifiuta {
				rifiutaLetture(t, "peers", "")
			}

			rec := invia("POST", guid.Replace("/api/ab/peer/add/PERSONALE"), `{"id":"`+tc.id+`","alias":"","tags":[]}`)
			if rec.Code != tc.stato || rec.Body.String() != tc.corpo {
				t.Errorf("POST /api/ab/peer/add: %d %s, attesi %d e %s", rec.Code, rec.Body, tc.stato, tc.corpo)
			}
			var voci []string
			if err := service.DB.Raw("SELECT hostname FROM address_books WHERE id = ?", tc.id).Scan(&voci).Error; err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(voci, ","); (tc.voce == "-" && len(voci) != 0) || (tc.voce != "-" && got != tc.voce) {
				t.Errorf("voci %s dopo l'aggiunta: %q, attesa %q", tc.id, voci, tc.voce)
			}
			if nelLog := registro.String(); tc.rifiuta && (!strings.Contains(nelLog, "POST /api/ab/peer/add/:guid: ") || !strings.Contains(nelLog, "lettura rifiutata dal test")) {
				t.Errorf("POST /api/ab/peer/add, nel log mancano rotta o errore:\n%s", nelLog)
			}
		})
	}
}
