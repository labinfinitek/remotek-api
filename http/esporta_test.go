package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// fileTrascrizione e' il file dell'esportazione come lo rilegge chi lo
// scarica: created_at e' una stringa, e la connessione resta grezza per
// confrontarla con la riga dell'elenco del pannello.
type fileTrascrizione struct {
	Connessione json.RawMessage
	Blocchi     []struct {
		Seq       int64
		Dir       string
		Data      []byte
		Hash      string
		Fine      bool
		CreatedAt string `json:"created_at"`
	}
	Verifica *model.AuditTerminalVerifica
}

// esporta scarica dal pannello il file della trascrizione della
// connessione id e lo restituisce letto, con il corpo grezzo.
func esporta(t *testing.T, g *gin.Engine, id uint) (fileTrascrizione, string) {
	t.Helper()
	rec := conToken(g, "GET", fmt.Sprint("/api/admin/audit_conn/terminal/export?audit_conn_id=", id), tokenDelPannello)
	if want, got := fmt.Sprintf(`attachment; filename="trascrizione-%d.json"`, id), rec.Header().Get("Content-Disposition"); rec.Code != 200 || got != want {
		t.Fatalf("esportazione: %d, Content-Disposition %q, attesi 200 e %q\n%s", rec.Code, got, want, rec.Body)
	}
	var f fileTrascrizione
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("file della trascrizione: %v\n%s", err, rec.Body)
	}
	return f, rec.Body.String()
}

// TestTrascrizioneEsportata prova sul router vero che il file della
// trascrizione abbia, in quest'ordine, la riga della connessione, gli
// stessi blocchi dell'elenco e la stessa verifica di verify, anche dopo
// una riga alterata nel database; e che nel log resti una riga info con
// l'amministratore e la connessione, senza contenuto.
func TestTrascrizioneEsportata(t *testing.T) {
	g, registro := terminaleDiProva(t, true, model.AuditConnTerminale)
	bb := catena("dir\r\n", contenuto, "exit\r\n")
	manda(t, g, bb...)

	f, corpo := esporta(t, g, 1)
	rec := conToken(g, "GET", "/api/admin/audit_conn/list", tokenDelPannello)
	var registroConn struct {
		Data struct{ List []json.RawMessage }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &registroConn); err != nil || len(registroConn.Data.List) != 1 {
		t.Fatalf("registro delle connessioni: %v %s", err, rec.Body)
	}
	if want, got := string(registroConn.Data.List[0]), string(f.Connessione); want != got {
		t.Errorf("connessione nel file: %s, attesa quella del registro %s", got, want)
	}
	if a, b, v := strings.Index(corpo, `"connessione"`), strings.Index(corpo, `"blocchi"`), strings.Index(corpo, `"verifica"`); a < 0 || a > b || b > v {
		t.Errorf("campi del file non nell'ordine connessione, blocchi, verifica:\n%s", corpo)
	}
	type riga struct {
		Seq  int64
		Dir  string
		Data string
		Hash string
		Fine bool
	}
	var nelFile, attese []riga
	for _, b := range f.Blocchi {
		nelFile = append(nelFile, riga{b.Seq, b.Dir, base64.StdEncoding.EncodeToString(b.Data), b.Hash, b.Fine})
		if b.CreatedAt == "" {
			t.Errorf("blocco %d senza created_at", b.Seq)
		}
	}
	for _, b := range bb {
		attese = append(attese, riga{b.Seq, b.Dir, b.Data, b.Hash, b.Fine})
	}
	if !reflect.DeepEqual(attese, nelFile) {
		t.Errorf("blocchi nel file: %+v, attesi %+v", nelFile, attese)
	}
	if want := verifica(t, g, 1); f.Verifica == nil || !reflect.DeepEqual(want, *f.Verifica) {
		t.Errorf("verifica nel file: %+v, attesa quella di verify %+v", f.Verifica, want)
	}
	nelLog := registro.String()
	if logger.Conta(nelLog, "INFO", "GET /api/admin/audit_conn/terminal/export: l'amministratore 1 ha esportato la trascrizione della connessione 1") != 1 {
		t.Errorf("manca la riga info dell'esportazione:\n%s", nelLog)
	}
	for _, c := range []string{contenuto, base64.StdEncoding.EncodeToString([]byte(contenuto))} {
		if strings.Contains(nelLog, c) {
			t.Errorf("nel log c'e' il contenuto %q:\n%s", c, nelLog)
		}
	}

	if err := service.DB.Exec("UPDATE audit_terminals SET data = ? WHERE seq = 2", []byte("altro")).Error; err != nil {
		t.Fatal(err)
	}
	f, _ = esporta(t, g, 1)
	want := model.AuditTerminalVerifica{Blocchi: 3, Fine: true, Hash: bb[2].Hash, PrimoErrato: 2}
	if f.Verifica == nil || !reflect.DeepEqual(want, *f.Verifica) || !reflect.DeepEqual(want, verifica(t, g, 1)) {
		t.Errorf("verifica dopo l'alterazione: %+v, attesa %+v anche da verify", f.Verifica, want)
	}
	if string(f.Blocchi[1].Data) != "altro" {
		t.Errorf("blocco 2 nel file: %q, atteso quello del database", f.Blocchi[1].Data)
	}
}

// TestTrascrizioneEsportataVuota prova sul router vero il file di una
// sessione terminale senza blocchi: elenco vuoto e verifica di zero blocchi.
func TestTrascrizioneEsportataVuota(t *testing.T) {
	g, _ := terminaleDiProva(t, true, model.AuditConnTerminale)
	f, corpo := esporta(t, g, 1)
	if want := (model.AuditTerminalVerifica{Integra: true}); f.Verifica == nil || !reflect.DeepEqual(want, *f.Verifica) || !strings.Contains(corpo, `"blocchi":[]`) {
		t.Errorf("file di una sessione senza blocchi:\n%s", corpo)
	}
}

// TestTrascrizioneEsportataRifiutata prova sul router vero che non escano
// file per chi non e' amministratore, per un id che non c'e' e per una
// connessione che non e' un terminale.
func TestTrascrizioneEsportataRifiutata(t *testing.T) {
	for _, tc := range []struct {
		nome   string
		admin  bool
		id     uint
		codice string
	}{
		{"non amministratore", false, 1, `"code":403`},
		{"id che non c'e'", true, 99, `"code":101`},
		{"connessione non terminale", true, 2, `"code":101`},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			g, _ := terminaleDiProva(t, tc.admin, model.AuditConnTerminale)
			crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 8, PeerId: "999000111"})
			manda(t, g, catena(contenuto)...)
			rec := conToken(g, "GET", fmt.Sprint("/api/admin/audit_conn/terminal/export?audit_conn_id=", tc.id), tokenDelPannello)
			body := rec.Body.String()
			if !strings.Contains(body, tc.codice) || rec.Header().Get("Content-Disposition") != "" || strings.Contains(body, "999000111") {
				t.Errorf("esportazione: %d %v %s, attesi %s e nessun file", rec.Code, rec.Header(), body, tc.codice)
			}
		})
	}
}

// scrittoreFermo e' la connessione di chi scarica piano: la prima scrittura
// della risposta si ferma finche' via non si chiude.
type scrittoreFermo struct {
	intestazione http.Header
	corpo        strings.Builder
	fermo, via   chan struct{}
	una          sync.Once
}

func (s *scrittoreFermo) Header() http.Header { return s.intestazione }
func (s *scrittoreFermo) WriteHeader(int)     {}
func (s *scrittoreFermo) Write(p []byte) (int, error) {
	s.una.Do(func() {
		close(s.fermo)
		<-s.via
	})
	return s.corpo.Write(p)
}

// TestTrascrizioneEsportataLenta prova sul router vero che l'esportazione
// non tenga il database mentre scrive a chi scarica: con l'API su una sola
// connessione SQLite, un download fermo bloccherebbe ogni altra richiesta.
// Mentre la risposta e' ferma, una lettura del database finisce; poi il
// file arriva intero, con blocchi su piu' pagine.
func TestTrascrizioneEsportataLenta(t *testing.T) {
	g, _ := terminaleDiProva(t, true, model.AuditConnTerminale)
	dati := make([]string, 40)
	for i := range dati {
		dati[i] = fmt.Sprint("riga ", i)
	}
	bb := catena(dati...)
	manda(t, g, bb...)

	s := &scrittoreFermo{intestazione: http.Header{}, fermo: make(chan struct{}), via: make(chan struct{})}
	req := httptest.NewRequest("GET", "/api/admin/audit_conn/terminal/export?audit_conn_id=1", nil)
	req.Header.Set("api-token", tokenDelPannello)
	finito := make(chan struct{})
	go func() {
		defer close(finito)
		g.ServeHTTP(s, req)
	}()
	<-s.fermo
	letto := make(chan error, 1)
	go func() {
		var n int64
		letto <- service.DB.Model(&model.AuditTerminal{}).Count(&n).Error
	}()
	select {
	case err := <-letto:
		if err != nil {
			t.Errorf("lettura del database durante l'esportazione: %v", err)
		}
		close(s.via)
	case <-time.After(3 * time.Second):
		t.Error("con l'esportazione ferma nella scrittura, una lettura del database non finisce in 3 s")
		close(s.via)
		<-letto
	}
	<-finito
	var f fileTrascrizione
	if err := json.Unmarshal([]byte(s.corpo.String()), &f); err != nil || len(f.Blocchi) != len(bb) {
		t.Fatalf("file dopo il download lento: %v, %d blocchi, attesi %d", err, len(f.Blocchi), len(bb))
	}
	if want := (model.AuditTerminalVerifica{Integra: true, Blocchi: 40, Fine: true, Hash: bb[39].Hash}); f.Verifica == nil || !reflect.DeepEqual(want, *f.Verifica) {
		t.Errorf("verifica nel file: %+v, attesa %+v", f.Verifica, want)
	}
}
