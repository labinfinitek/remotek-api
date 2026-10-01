package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// contenuto e' il testo dei blocchi di prova: non deve mai finire nel log,
// ne' in chiaro ne' in base64.
const contenuto = "password-del-cliente"

// blocco e' un blocco della trascrizione come lo manda il PC.
type blocco struct {
	Id     string `json:"id"`
	Uuid   string `json:"uuid"`
	ConnId int64  `json:"conn_id"`
	Seq    int64  `json:"seq"`
	Dir    string `json:"dir"`
	Data   string `json:"data"`
	Hash   string `json:"hash"`
	Fine   bool   `json:"fine"`
}

// catena restituisce i blocchi 1..len(dati) della connessione 7 di
// 999000111, con dir alternati a partire da "in", hash concatenati come nel
// README e fine sull'ultimo.
func catena(dati ...string) []blocco {
	var bb []blocco
	for i, d := range dati {
		b := blocco{Id: "999000111", Uuid: uuidSalvato, ConnId: 7, Seq: int64(i + 1), Dir: []string{"in", "out"}[i%2],
			Data: base64.StdEncoding.EncodeToString([]byte(d)), Fine: i == len(dati)-1}
		firma(&b, bb)
		bb = append(bb, b)
	}
	return bb
}

// firma scrive in b l'hash che gli spetta dopo i blocchi prima, calcolato
// con il primo byte di b.Dir, qualunque sia: cosi' un blocco con un dir o
// una lunghezza sbagliati ha l'hash giusto, e lo scarta solo il controllo
// che lo riguarda.
func firma(b *blocco, prima []blocco) {
	prec := make([]byte, sha256.Size)
	if len(prima) > 0 {
		prec, _ = hex.DecodeString(prima[len(prima)-1].Hash)
	}
	data, _ := base64.StdEncoding.DecodeString(b.Data)
	h := sha256.Sum256(append(append(append([]byte{}, prec...), b.Dir[0]), data...))
	b.Hash = hex.EncodeToString(h[:])
}

// terminaleDiProva prepara il router vero con il PC 999000111 e la sua
// connessione 7 di tipo tipo; admin dice se l'utente del pannello lo e'.
func terminaleDiProva(t *testing.T, admin bool, tipo int) (*gin.Engine, *strings.Builder) {
	t.Helper()
	g, _, registro := pannello(t, admin)
	if err := service.DB.AutoMigrate(&model.Peer{}, &model.AuditConn{}, &model.AuditTerminal{}); err != nil {
		t.Fatal(err)
	}
	crea(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", Type: tipo})
	return g, registro
}

// manda manda i blocchi bb a /api/audit/terminal e controlla la risposta
// delle rotte dell'audit.
func manda(t *testing.T, g *gin.Engine, bb ...blocco) {
	t.Helper()
	for _, b := range bb {
		corpo, _ := json.Marshal(b)
		rec := richiesta(g, "POST", "/api/audit/terminal", "", string(corpo))
		if got, want := rec.Body.String(), `{"code":0,"message":"success","data":""}`; rec.Code != 200 || got != want {
			t.Errorf("blocco %d: %d %s, attesi 200 e %s", b.Seq, rec.Code, got, want)
		}
	}
}

// salvati restituisce i seq salvati della trascrizione, in ordine.
func salvati(t *testing.T) []int64 {
	t.Helper()
	var seq []int64
	if err := service.DB.Raw("SELECT seq FROM audit_terminals ORDER BY seq").Scan(&seq).Error; err != nil {
		t.Fatal(err)
	}
	return seq
}

// verifica chiede al pannello la verifica della trascrizione della
// connessione id.
func verifica(t *testing.T, g *gin.Engine, id uint) model.AuditTerminalVerifica {
	t.Helper()
	rec := conToken(g, "GET", fmt.Sprint("/api/admin/audit_conn/terminal/verify?audit_conn_id=", id), tokenDelPannello)
	var r struct {
		Code int
		Data model.AuditTerminalVerifica
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Code != 0 {
		t.Fatalf("verifica: %d %s", rec.Code, rec.Body)
	}
	return r.Data
}

// TestTrascrizioneTerminale prova sul router vero che i blocchi in ordine
// si salvano, che il pannello li elenca in ordine di seq con data in base64
// e che la verifica li trova integri; poi che una riga alterata nel
// database fa dire non integra dal suo seq.
func TestTrascrizioneTerminale(t *testing.T) {
	g, registro := terminaleDiProva(t, true, model.AuditConnTerminale)
	bb := catena("dir\r\n", contenuto, "exit\r\n")
	manda(t, g, bb...)

	rec := conToken(g, "GET", "/api/admin/audit_conn/terminal/list?audit_conn_id=1", tokenDelPannello)
	type riga struct {
		PeerId string `json:"peer_id"`
		ConnId int64  `json:"conn_id"`
		Seq    int64  `json:"seq"`
		Dir    string `json:"dir"`
		Data   string `json:"data"`
		Hash   string `json:"hash"`
		Fine   bool   `json:"fine"`
	}
	var elenco struct{ Data struct{ List []riga } }
	if err := json.Unmarshal(rec.Body.Bytes(), &elenco); err != nil {
		t.Fatalf("elenco: %v %s", err, rec.Body)
	}
	var attese []riga
	for _, b := range bb {
		attese = append(attese, riga{b.Id, b.ConnId, b.Seq, b.Dir, b.Data, b.Hash, b.Fine})
	}
	if want, got := attese, elenco.Data.List; !reflect.DeepEqual(want, got) {
		t.Errorf("elenco dei blocchi: %+v, attesi %+v", got, want)
	}
	if want, got := (model.AuditTerminalVerifica{Integra: true, Blocchi: 3, Fine: true, Hash: bb[2].Hash}), verifica(t, g, 1); !reflect.DeepEqual(want, got) {
		t.Errorf("verifica: %+v, attesi %+v", got, want)
	}
	if nelLog := registro.String(); nelLog != "" {
		t.Errorf("blocchi validi, nel log:\n%s", nelLog)
	}

	if err := service.DB.Exec("UPDATE audit_terminals SET data = ? WHERE seq = 2", []byte("altro")).Error; err != nil {
		t.Fatal(err)
	}
	if want, got := (model.AuditTerminalVerifica{Blocchi: 3, Fine: true, Hash: bb[2].Hash, PrimoErrato: 2}), verifica(t, g, 1); !reflect.DeepEqual(want, got) {
		t.Errorf("verifica dopo l'alterazione: %+v, attesi %+v", got, want)
	}
}

// TestTrascrizioneRifiutata prova sul router vero i blocchi che non passano
// i controlli: niente salvato oltre ai blocchi validi di prima, e un warn
// con rotta e ID del PC, senza uuid ne' contenuto.
func TestTrascrizioneRifiutata(t *testing.T) {
	// 64 KiB + 1 byte ha in base64 la stessa lunghezza di 64 KiB: lo scarta
	// il controllo dopo la decodifica; con altri 4 caratteri quello prima.
	grande := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", model.BloccoTerminaleMax+1)))
	for _, tc := range []struct {
		nome, motivo string
		tipo         int
		prima        int  // blocchi validi mandati prima
		rifirma      bool // dopo cambia, b ha l'hash giusto per i suoi dati e il suo dir
		cambia       func(b *blocco)
	}{
		{"uuid di un altro PC", "uuid diverso da quello salvato", model.AuditConnTerminale, 0, false, func(b *blocco) { b.Uuid = uuidAltro }},
		{"conn_id senza connessione", "nessuna connessione terminale con questo conn_id", model.AuditConnTerminale, 0, false, func(b *blocco) { b.ConnId = 8 }},
		{"connessione non terminale", "nessuna connessione terminale con questo conn_id", 0, 0, false, func(*blocco) {}},
		{"dir sbagliato", "dir sconosciuto", model.AuditConnTerminale, 0, true, func(b *blocco) { b.Dir = "err" }},
		{"blocco vuoto senza fine", "blocco vuoto senza fine", model.AuditConnTerminale, 0, true, func(b *blocco) { b.Data = "" }},
		{"seq oltre 100000", "trascrizione oltre 100000 blocchi", model.AuditConnTerminale, 1, false, func(b *blocco) { b.Seq = model.BlocchiTerminaleMax + 1 }},
		{"base64 rotto", "data non e' base64", model.AuditConnTerminale, 0, false, func(b *blocco) { b.Data = "!!" + b.Data }},
		{"blocco oltre 64 KiB", "blocco oltre 64 KiB", model.AuditConnTerminale, 0, true, func(b *blocco) { b.Data = grande }},
		{"blocco oltre 64 KiB, stringa lunga", "blocco oltre 64 KiB", model.AuditConnTerminale, 0, true, func(b *blocco) { b.Data = grande + "AAAA" }},
		{"seq saltato", "seq 3 invece di 2", model.AuditConnTerminale, 1, false, func(b *blocco) { b.Seq = 3 }},
		{"seq ripetuto", "seq 1 invece di 2", model.AuditConnTerminale, 1, false, func(b *blocco) { b.Seq = 1 }},
		{"hash sbagliato", "hash del blocco 2 sbagliato", model.AuditConnTerminale, 1, false, func(b *blocco) { b.Hash = strings.Repeat("0", 64) }},
		{"blocco dopo fine", "blocco 3 dopo la fine della sessione", model.AuditConnTerminale, 2, false, func(*blocco) {}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, registro := terminaleDiProva(t, false, tc.tipo)
			bb := catena("uno", contenuto, contenuto)
			if tc.prima == 2 {
				bb = append(catena("uno", contenuto), bb[2])
			}
			manda(t, g, bb[:tc.prima]...)
			b := bb[tc.prima]
			tc.cambia(&b)
			if tc.rifirma {
				firma(&b, bb[:tc.prima])
			}
			manda(t, g, b)

			var attesi []int64
			for i := range tc.prima {
				attesi = append(attesi, int64(i+1))
			}
			if want, got := attesi, salvati(t); !reflect.DeepEqual(want, got) {
				t.Errorf("seq salvati: %+v, attesi %+v", got, want)
			}
			senzaUuid(t, registro, "/api/audit/terminal")
			nelLog := registro.String()
			if logger.Conta(nelLog, "WARN", "niente salvato", tc.motivo) != 1 {
				t.Errorf("atteso un warn con %q:\n%s", tc.motivo, nelLog)
			}
			for _, c := range []string{contenuto, base64.StdEncoding.EncodeToString([]byte(contenuto)), b.Hash} {
				if strings.Contains(nelLog, c) {
					t.Errorf("nel log c'e' il contenuto %q:\n%s", c, nelLog)
				}
			}
		})
	}
}

// TestTrascrizioneOltreIlLimite prova sul router vero che una sessione non
// passa i 20 MiB: il blocco che arriva esattamente al limite si salva col
// totale giusto, quello dopo non si salva e va nel log. Il conto vale anche
// se le righe prima non hanno il totale, come quelle salvate prima della
// colonna.
func TestTrascrizioneOltreIlLimite(t *testing.T) {
	for _, senzaTotale := range []bool{false, true} {
		t.Run(fmt.Sprint("senza totale ", senzaTotale), func(t *testing.T) {
			g, registro := terminaleDiProva(t, false, model.AuditConnTerminale)
			pieno := strings.Repeat("x", model.BloccoTerminaleMax)
			dati := make([]string, model.TrascrizioneTerminale/model.BloccoTerminaleMax+1)
			for i := range dati {
				dati[i] = pieno
			}
			dati[len(dati)-1] = "x"
			bb := catena(dati...)
			for i := range bb {
				bb[i].Fine = false
			}
			n := len(bb)
			manda(t, g, bb[:n-2]...)
			if senzaTotale {
				if err := service.DB.Exec("UPDATE audit_terminals SET totale = NULL").Error; err != nil {
					t.Fatal(err)
				}
			}
			manda(t, g, bb[n-2:]...)
			if got := len(salvati(t)); got != n-1 {
				t.Errorf("salvati %d blocchi, attesi %d", got, n-1)
			}
			var totale int64
			if err := service.DB.Raw("SELECT totale FROM audit_terminals WHERE seq = ?", n-1).Scan(&totale).Error; err != nil {
				t.Fatal(err)
			}
			if totale != model.TrascrizioneTerminale {
				t.Errorf("totale del blocco %d: %d, attesi %d", n-1, totale, model.TrascrizioneTerminale)
			}
			if nelLog := registro.String(); logger.Conta(nelLog, "WARN", "POST /api/audit/terminal: ", "999000111", "oltre 20 MiB") != 1 {
				t.Errorf("nel log manca il warn sul limite:\n%s", nelLog)
			}
		})
	}
}

// TestTrascrizioneFineVuota prova sul router vero che un blocco vuoto con
// fine chiude la sessione: si salva e la verifica la trova integra e finita.
func TestTrascrizioneFineVuota(t *testing.T) {
	g, registro := terminaleDiProva(t, true, model.AuditConnTerminale)
	bb := catena(contenuto, "")
	manda(t, g, bb...)
	if want, got := []int64{1, 2}, salvati(t); !reflect.DeepEqual(want, got) {
		t.Errorf("seq salvati: %+v, attesi %+v", got, want)
	}
	if want, got := (model.AuditTerminalVerifica{Integra: true, Blocchi: 2, Fine: true, Hash: bb[1].Hash}), verifica(t, g, 1); !reflect.DeepEqual(want, got) {
		t.Errorf("verifica: %+v, attesi %+v", got, want)
	}
	if nelLog := registro.String(); nelLog != "" {
		t.Errorf("blocchi validi, nel log:\n%s", nelLog)
	}
}

// TestTrascrizioneSoloAdmin prova sul router vero che elenco e verifica
// della trascrizione rispondono NoAccess a chi non e' amministratore.
func TestTrascrizioneSoloAdmin(t *testing.T) {
	g, _ := terminaleDiProva(t, false, model.AuditConnTerminale)
	manda(t, g, catena(contenuto)...)
	for _, rotta := range []string{"/api/admin/audit_conn/terminal/list", "/api/admin/audit_conn/terminal/verify"} {
		rec := conToken(g, "GET", rotta+"?audit_conn_id=1", tokenDelPannello)
		if body := rec.Body.String(); !strings.Contains(body, `"code":403`) || strings.Contains(body, "999000111") {
			t.Errorf("GET %s da non amministratore: %d %s", rotta, rec.Code, body)
		}
	}
}

// TestTrascrizioneConnIdRipetuto prova sul router vero due sessioni
// terminale dello stesso PC con lo stesso conn_id, come dopo un riavvio del
// servizio: i blocchi della seconda si salvano da seq 1 sulla connessione
// piu' recente, la verifica di ciascuna risponde integra col suo hash
// finale, e cancellare la prima dal pannello, da sola o in blocco, lascia la
// trascrizione della seconda. Un conn_id la cui ultima connessione non e'
// un terminale non salva niente.
func TestTrascrizioneConnIdRipetuto(t *testing.T) {
	for _, rotta := range []string{"/api/admin/audit_conn/delete", "/api/admin/audit_conn/batchDelete"} {
		t.Run(rotta, func(t *testing.T) {
			g, registro := terminaleDiProva(t, true, model.AuditConnTerminale)
			prima := catena("uno", contenuto)
			manda(t, g, prima...)
			crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", Type: model.AuditConnTerminale})
			seconda := catena("due", "tre", "quattro")
			manda(t, g, seconda...)

			for id, bb := range map[uint][]blocco{1: prima, 2: seconda} {
				want := model.AuditTerminalVerifica{Integra: true, Blocchi: int64(len(bb)), Fine: true, Hash: bb[len(bb)-1].Hash}
				if got := verifica(t, g, id); !reflect.DeepEqual(want, got) {
					t.Errorf("verifica della connessione %d: %+v, attesa %+v", id, got, want)
				}
			}
			if nelLog := registro.String(); nelLog != "" {
				t.Errorf("blocchi validi, nel log:\n%s", nelLog)
			}

			corpo := `{"id":1}`
			if strings.HasSuffix(rotta, "batchDelete") {
				corpo = `{"ids":[1]}`
			}
			if rec := alPannello(g, rotta, corpo); !strings.Contains(rec.Body.String(), `"code":0`) {
				t.Fatalf("POST %s: %s", rotta, rec.Body)
			}
			var resta []string
			if err := service.DB.Raw("SELECT audit_conn_id || '/' || seq FROM audit_terminals ORDER BY seq").Scan(&resta).Error; err != nil {
				t.Fatal(err)
			}
			if want := []string{"2/1", "2/2", "2/3"}; !reflect.DeepEqual(want, resta) {
				t.Errorf("blocchi rimasti: %v, attesi %v", resta, want)
			}

			crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111"})
			manda(t, g, catena("cinque")...)
			if n := righe(t, "audit_terminals"); n != 3 {
				t.Errorf("audit_terminals ha %d righe dopo un blocco per una connessione non terminale, attese 3", n)
			}
		})
	}
}
