package http

// Test delle letture dei dispositivi e dei gruppi sul router vero: un errore
// del database non vale "dispositivo nuovo", "gruppo non condiviso" o un
// elenco vuoto.

import (
	"strconv"
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

// TestClientGruppiNonLetti prova sul router vero le rotte del client che
// leggono il gruppo dell'utente, i gruppi di dispositivi e i dispositivi del
// gruppo. L'utente di pannello sta in un gruppo condiviso col proprietario,
// che ha il dispositivo 999000555 nel gruppo di dispositivi reparto. Se il
// database non legge una di queste righe le rotte rispondono 400 {"error":
// "Errore di sistema."}, con l'errore nel log, e non 401, che per il client
// e' un logout: il client 1.4.9 tiene gli elenchi di prima. Prima
// rispondevano 200 con un elenco ridotto (il gruppo non letto valeva "non
// condiviso": solo l'utente stesso) o vuoto, che il client salva nella
// cache. Senza ostacoli le stesse richieste rispondono 200 coi dati.
func TestClientGruppiNonLetti(t *testing.T) {
	for _, tc := range []struct {
		nome, rotta, nelLog string
		tabella, dove       string // la lettura che fallisce; dove "-": l'elenco senza WHERE
		amministratore      bool   // solo l'amministratore legge i gruppi di dispositivi
		dato                string // nella risposta senza ostacoli
	}{
		{"utenti, gruppo", "/api/users", "GET /api/users: ", "groups", "", false, "proprietario"},
		{"dispositivi, gruppo", "/api/peers", "GET /api/peers: ", "groups", "", false, "999000555"},
		{"dispositivi, gruppi di dispositivi", "/api/peers", "GET /api/peers: ", "device_groups", "-", false, "reparto"},
		{"dispositivi, dispositivi", "/api/peers", "GET /api/peers: ", "peers", "user_id in", false, "999000555"},
		{"gruppi di dispositivi", "/api/device-group/accessible", "GET /api/device-group/accessible: ", "device_groups", "-", true, "reparto"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			_, registro, invia, _ := rubricheDiProva(t, model.ShareAddressBookRuleRuleRead)
			if err := service.DB.AutoMigrate(&model.Group{}, &model.DeviceGroup{}); err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Group{IdModel: model.IdModel{Id: 1}, Name: "condiviso", Type: model.GroupTypeShare})
			reparto := &model.DeviceGroup{Name: "reparto"}
			crea(t, reparto)
			var proprietario uint
			if err := service.DB.Raw("SELECT id FROM users WHERE username = 'proprietario'").Scan(&proprietario).Error; err != nil {
				t.Fatal(err)
			}
			crea(t, &model.Peer{Id: "999000555", UserId: proprietario, GroupId: reparto.Id})
			if tc.amministratore {
				if err := service.DB.Exec("UPDATE users SET is_admin = 1 WHERE username = 'prova'").Error; err != nil {
					t.Fatal(err)
				}
			}
			if rec := invia("GET", tc.rotta, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.dato) {
				t.Fatalf("GET %s senza ostacoli: %d %s, atteso 200 con %s", tc.rotta, rec.Code, rec.Body, tc.dato)
			}
			if tc.dove == "-" {
				rifiutaElenchi(t, tc.tabella)
			} else {
				rifiutaLetture(t, tc.tabella, tc.dove)
			}
			registro.Reset()

			rec := invia("GET", tc.rotta, "")
			if got, want := rec.Body.String(), `{"error":"Errore di sistema."}`; rec.Code != 400 || got != want {
				t.Errorf("GET %s con %s non letti: %d %s, attesi 400 e %s", tc.rotta, tc.tabella, rec.Code, got, want)
			}
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.nelLog) || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("GET %s, nel log mancano rotta o errore:\n%s", tc.rotta, nelLog)
			}
		})
	}
}

// TestPannelloDispositiviEGruppiNonLetti prova sul router vero le rotte del
// pannello che leggono dispositivi, gruppi di utenti e gruppi di
// dispositivi. Se il database non li legge rispondono code 101 "Errore di
// sistema.", con l'errore nel log, e non cambiano niente. Prima dettaglio e
// cancellazione dicevano "Elemento non trovato.", gli elenchi erano vuoti, e
// l'aggiunta in rubrica dai dispositivi e la regola di condivisione verso un
// gruppo dicevano "Elemento non trovato.". Una riga che non c'e' ha la
// risposta di prima.
func TestPannelloDispositiviEGruppiNonLetti(t *testing.T) {
	const erroreDiSistema = `{"code":101,"message":"Errore di sistema.","data":null}`
	const nonTrovato = `{"code":101,"message":"Elemento non trovato.","data":null}`
	const regola = `{"user_id":UTENTE,"collection_id":RUBRICA,"type":2,"rule":2,"to_id":`
	for _, tc := range []struct {
		metodo, rotta, corpo string // PEER, GRUPPO, REPARTO, UTENTE e RUBRICA diventano gli id
		tabella, dove        string // le letture rifiutate: tutte quelle senza WHERE se dove e' "elenco"
		risposta             string
	}{
		{"GET", "/api/admin/peer/detail/PEER", "", "peers", "row_id", erroreDiSistema},
		{"POST", "/api/admin/peer/delete", `{"row_id":PEER}`, "peers", "row_id", erroreDiSistema},
		{"GET", "/api/admin/peer/list", "", "peers", "elenco", erroreDiSistema},
		{"POST", "/api/admin/peer/simpleData", `{"ids":["999000111"]}`, "peers", "id in", erroreDiSistema},
		{"GET", "/api/admin/my/peer/list", "", "peers", "user_id", erroreDiSistema},
		{"POST", "/api/admin/address_book/batchCreateFromPeers", `{"user_id":UTENTE,"peer_ids":[PEER]}`, "peers", "row_id in", erroreDiSistema},
		{"POST", "/api/admin/my/address_book/batchCreateFromPeers", `{"peer_ids":[PEER]}`, "peers", "row_id in", erroreDiSistema},
		{"GET", "/api/admin/group/detail/GRUPPO", "", "groups", "id = ?", erroreDiSistema},
		{"POST", "/api/admin/group/delete", `{"id":GRUPPO}`, "groups", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/group/list", "", "groups", "elenco", erroreDiSistema},
		{"POST", "/api/admin/user/groupUsers", "", "groups", "elenco", erroreDiSistema},
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `GRUPPO}`, "groups", "id = ?", erroreDiSistema},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `GRUPPO}`, "groups", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/device_group/detail/REPARTO", "", "device_groups", "id = ?", erroreDiSistema},
		{"POST", "/api/admin/device_group/delete", `{"id":REPARTO}`, "device_groups", "id = ?", erroreDiSistema},
		{"GET", "/api/admin/device_group/list", "", "device_groups", "elenco", erroreDiSistema},
		{"GET", "/api/admin/peer/detail/999999", "", "", "", nonTrovato},
		{"POST", "/api/admin/peer/delete", `{"row_id":999999}`, "", "", nonTrovato},
		{"GET", "/api/admin/group/detail/999999", "", "", "", nonTrovato},
		{"POST", "/api/admin/group/delete", `{"id":999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/address_book_collection_rule/create", regola + `999999}`, "", "", nonTrovato},
		{"POST", "/api/admin/my/address_book_collection_rule/create", regola + `999999}`, "", "", nonTrovato},
		{"GET", "/api/admin/device_group/detail/999999", "", "", "", nonTrovato},
		{"POST", "/api/admin/device_group/delete", `{"id":999999}`, "", "", nonTrovato},
	} {
		t.Run(tc.metodo+" "+tc.rotta+" "+tc.tabella, func(t *testing.T) {
			g, utente, registro := pannello(t, true)
			err := service.DB.AutoMigrate(&model.Peer{}, &model.Group{}, &model.DeviceGroup{},
				&model.AddressBook{}, &model.AddressBookCollection{}, &model.AddressBookCollectionRule{})
			if err != nil {
				t.Fatal(err)
			}
			peer := &model.Peer{Id: "999000111", UserId: utente.Id}
			crea(t, peer)
			gruppo := &model.Group{Name: "condiviso", Type: model.GroupTypeShare}
			crea(t, gruppo)
			reparto := &model.DeviceGroup{Name: "reparto"}
			crea(t, reparto)
			rubrica := &model.AddressBookCollection{UserId: utente.Id, Name: "ufficio"}
			crea(t, rubrica)
			id := func(n uint) string { return strconv.FormatUint(uint64(n), 10) }
			sostituisci := strings.NewReplacer("PEER", id(peer.RowId), "GRUPPO", id(gruppo.Id), "REPARTO", id(reparto.Id),
				"UTENTE", id(utente.Id), "RUBRICA", id(rubrica.Id))
			prima := statoDispositivi(t)
			switch tc.dove {
			case "":
			case "elenco":
				rifiutaElenchi(t, tc.tabella)
			default:
				rifiutaLetture(t, tc.tabella, tc.dove)
			}

			rec := richiesta(g, tc.metodo, sostituisci.Replace(tc.rotta), "", sostituisci.Replace(tc.corpo))
			if got := rec.Body.String(); rec.Code != 200 || got != tc.risposta {
				t.Errorf("%s %s: stato %d\n got  %s\n want %s", tc.metodo, tc.rotta, rec.Code, got, tc.risposta)
			}
			if tc.dove == "" {
				return
			}
			if dopo := statoDispositivi(t); dopo != prima {
				t.Errorf("%s %s ha cambiato il database:\n prima %s\n dopo  %s", tc.metodo, tc.rotta, prima, dopo)
			}
			percorso := strings.NewReplacer("/PEER", "/:id", "/GRUPPO", "/:id", "/REPARTO", "/:id").Replace(tc.rotta)
			if nelLog := registro.String(); !strings.Contains(nelLog, tc.metodo+" "+percorso+": ") || !strings.Contains(nelLog, "lettura rifiutata dal test") {
				t.Errorf("%s %s, nel log mancano rotta o errore:\n%s", tc.metodo, tc.rotta, nelLog)
			}
		})
	}
}

// statoDispositivi conta con Raw, che rifiutaLetture non ferma, dispositivi,
// gruppi, gruppi di dispositivi, voci e regole di condivisione.
func statoDispositivi(t *testing.T) string {
	t.Helper()
	var stato []string
	for _, tabella := range []string{"peers", "groups", "device_groups", "address_books", "address_book_collection_rules"} {
		stato = append(stato, tabella+" "+strconv.FormatInt(righe(t, tabella), 10))
	}
	return strings.Join(stato, ", ")
}
