package http

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// sessioneDelClient manda sul router vero cio' che i client mandano in una
// sessione terminale: new all'apertura del socket, autorizzazione con
// tecnico e type 4 due volte uguale, nota durante la sessione due volte
// uguale, nota di fine connessione col guid, chiusura; poi una connessione
// mai autorizzata, new e chiusura. Ogni risposta deve restare quella di
// sempre.
func sessioneDelClient(t *testing.T, g *gin.Engine) {
	t.Helper()
	const ok = `{"code":0,"message":"success","data":""}`
	manda := func(metodo, rotta, token, corpo string) {
		t.Helper()
		if rec := daTecnico(g, metodo, rotta, token, corpo); rec.Code != 200 || rec.Body.String() != ok {
			t.Errorf("%s %s: %d %s, attesi 200 e %s", metodo, rotta, rec.Code, rec.Body, ok)
		}
	}
	manda("POST", "/api/audit/conn", "", `{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"`+uuidSalvato+`"}`)
	for range 2 {
		manda("POST", "/api/audit/conn", "", `{"conn_id":7,"id":"999000111","peer":["111222333","mario"],"session_id":1,"type":4,"uuid":"`+uuidSalvato+`"}`)
	}
	manda("POST", "/api/audit/conn", "", `{"id":"999000111","note":"#83100","session_id":1}`)
	manda("POST", "/api/audit/conn", "", `{"id":"999000111","note":"#83100","session_id":1}`)
	var guid string
	if err := service.DB.Raw("SELECT guid FROM audit_conns WHERE conn_id = 7").Scan(&guid).Error; err != nil {
		t.Fatal(err)
	}
	manda("PUT", "/api/audit", tokenDelPannello, `{"guid":"`+guid+`","note":"#83100 risolto"}`)
	manda("POST", "/api/audit/conn", "", `{"action":"close","conn_id":7,"id":"999000111","session_id":1,"uuid":"`+uuidSalvato+`"}`)
	manda("POST", "/api/audit/conn", "", `{"action":"new","conn_id":8,"id":"999000111","ip":"192.0.2.10","session_id":0,"uuid":"`+uuidSalvato+`"}`)
	manda("POST", "/api/audit/conn", "", `{"action":"close","conn_id":8,"id":"999000111","session_id":0,"uuid":"`+uuidSalvato+`"}`)
}

// integrazioneDiProva prepara il router con la tabella della coda e
// integrazione.url impostato se url non e' vuoto.
func integrazioneDiProva(t *testing.T, url string) (*gin.Engine, *strings.Builder) {
	t.Helper()
	g, registro := connessioniDiProva(t)
	if err := service.DB.AutoMigrate(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	prec := global.Config.Integrazione
	t.Cleanup(func() { global.Config.Integrazione = prec })
	global.Config.Integrazione = config.Integrazione{Url: url, Segreto: strings.Repeat("s", 32)}
	return g, registro
}

// TestEventiDelleConnessioni: con l'integrazione accesa la sessione
// autorizzata accoda una sola apertura, all'autorizzazione, con tecnico e
// tipo 4; le due note diverse (non la nota ripetuta uguale) e la chiusura
// con la durata, nell'ordine. La connessione mai autorizzata ha solo la
// chiusura, con tecnico vuoto e tipo 0.
func TestEventiDelleConnessioni(t *testing.T) {
	g, _ := integrazioneDiProva(t, "https://n8n.example/webhook/remotek")
	sessioneDelClient(t, g)
	var coda []model.Evento
	if err := service.DB.Order("id").Find(&coda).Error; err != nil {
		t.Fatal(err)
	}
	var tipi, note, tecnici []string
	var tipiSessione []int
	for _, e := range coda {
		var c struct {
			Sessione service.SessioneEvento
		}
		if err := json.Unmarshal([]byte(e.Corpo), &c); err != nil {
			t.Fatal(err)
		}
		tipi, note = append(tipi, e.Tipo), append(note, c.Sessione.Nota)
		tecnici, tipiSessione = append(tecnici, c.Sessione.Tecnico), append(tipiSessione, c.Sessione.Tipo)
		if strings.HasPrefix(c.Sessione.Inizio, "0001") || c.Sessione.Inizio == "" {
			t.Errorf("%s senza l'ora d'inizio: %s", e.Tipo, e.Corpo)
		}
		if e.Tipo == service.EventoConnessioneChiusa && (c.Sessione.DurataS == nil || c.Sessione.Fine == nil) {
			t.Errorf("chiusura senza durata o fine: %s", e.Corpo)
		}
	}
	wantTipi := []string{service.EventoConnessioneAperta, service.EventoConnessioneNota, service.EventoConnessioneNota,
		service.EventoConnessioneChiusa, service.EventoConnessioneChiusa}
	wantNote := []string{"", "#83100", "#83100 risolto", "#83100 risolto", ""}
	wantTecnici := []string{"mario", "mario", "mario", "mario", ""}
	if !slices.Equal(tipi, wantTipi) || !slices.Equal(note, wantNote) || !slices.Equal(tecnici, wantTecnici) ||
		!slices.Equal(tipiSessione, []int{4, 4, 4, 4, 0}) {
		t.Errorf("eventi %v, note %q, tecnici %q, tipi %v; attesi %v, %q, %q, [4 4 4 4 0]", tipi, note, tecnici, tipiSessione, wantTipi, wantNote, wantTecnici)
	}
}

// TestEventiSenzaUrl: senza integrazione.url la stessa sessione non
// accoda niente.
func TestEventiSenzaUrl(t *testing.T) {
	g, _ := integrazioneDiProva(t, "")
	sessioneDelClient(t, g)
	var n int64
	if err := service.DB.Model(&model.Evento{}).Count(&n).Error; err != nil || n != 0 {
		t.Errorf("%d eventi in coda (%v), attesi 0", n, err)
	}
}

// TestEventoNonAccodato: se la coda non si scrive, il client riceve la
// risposta di sempre e nel log c'e' un warn per ogni evento perso.
func TestEventoNonAccodato(t *testing.T) {
	g, registro := integrazioneDiProva(t, "https://n8n.example/webhook/remotek")
	if err := service.DB.Migrator().DropTable(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	sessioneDelClient(t, g)
	if n := logger.Conta(registro.String(), "WARN", "non accodato"); n != 5 {
		t.Errorf("%d warn, attesi 5:\n%s", n, registro)
	}
}
