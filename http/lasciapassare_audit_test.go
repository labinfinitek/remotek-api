package http

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// TestAuditConLasciapassare prova sul router vero POST /api/audit/conn coi
// campi lasciapassare e amministratore (ADR-0028). Il nonce giusto, mandato
// col new, lega il lasciapassare alla connessione 7 e la segna con agente e
// tecnico; l'autorizzazione, con amministratore falso ("accetta come
// utente") e un altro nonce valido, non la rilega e non da' warn. Aperta e
// chiusa portano agente, la nota no. Amministratore col solo new (14) vale. Un altro nonce giusto con amministratore vero (12) da' agente e
// amministratore; amministratore vero senza nonce (13) solo amministratore.
// Il nonce gia' usato (riusato sulla 8), quello di un altro PC e uno
// sconosciuto danno un warn senza il nonce, e la riga si salva come senza
// il campo; senza campi (11) la riga di sempre. Le risposte al PC sono
// quelle di sempre.
func TestAuditConLasciapassare(t *testing.T) {
	g, registro := integrazioneDiProva(t, "https://n8n.example/webhook")
	if err := service.DB.AutoMigrate(&model.Lasciapassare{}); err != nil {
		t.Fatal(err)
	}
	no := false
	crea(t, &model.User{Username: "mario", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE})
	crea(t, &model.User{Username: "agente-mario", GroupId: 1, IsAdmin: &no, Status: model.COMMON_STATUS_ENABLE, AgenteDi: 2})
	for _, lp := range []*model.Lasciapassare{
		{Nonce: "nonce-giusto", Pc: "999000111", AgenteId: 3, TecnicoId: 2, Emesso: 1, Scade: 61, Creato: 1},
		{Nonce: "nonce-altro-pc", Pc: "999000222", AgenteId: 3, TecnicoId: 2, Emesso: 1, Scade: 61, Creato: 1},
		{Nonce: "nonce-secondo", Pc: "999000111", AgenteId: 3, TecnicoId: 2, Emesso: 1, Scade: 61, Creato: 1},
		{Nonce: "nonce-terzo", Pc: "999000111", AgenteId: 3, TecnicoId: 2, Emesso: 1, Scade: 61, Creato: 1},
	} {
		crea(t, lp)
	}
	const ok = `{"code":0,"message":"success","data":""}`
	manda := func(corpo string) {
		t.Helper()
		if rec := daTecnico(g, "POST", "/api/audit/conn", "", corpo); rec.Code != 200 || rec.Body.String() != ok {
			t.Errorf("%s: %d %s, attesi 200 e %s", corpo, rec.Code, rec.Body, ok)
		}
	}
	// conn manda new e autorizzazione della connessione id; ammin, se non
	// vuoto, e' il campo amministratore dell'autorizzazione.
	conn := func(id int, lp, ammin string) {
		t.Helper()
		campo := ""
		if lp != "" {
			campo = `,"lasciapassare":"` + lp + `"`
		}
		auth := campo
		if ammin != "" {
			auth += `,"amministratore":` + ammin
		}
		manda(`{"action":"new","conn_id":` + itoa(id) + `,"id":"999000111","ip":"192.0.2.10","session_id":` + itoa(id) + `,"uuid":"` + uuidSalvato + `"` + campo + `}`)
		manda(`{"conn_id":` + itoa(id) + `,"id":"999000111","peer":["111222333","agente-mario"],"session_id":` + itoa(id) + `,"type":0,"uuid":"` + uuidSalvato + `"` + auth + `}`)
	}
	// La 7 ha il nonce giusto col new; all'autorizzazione arriva un altro
	// nonce valido dello stesso PC, che non si lega: la connessione e' gia'
	// segnata.
	manda(`{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":7,"uuid":"` + uuidSalvato + `","lasciapassare":"nonce-giusto"}`)
	manda(`{"conn_id":7,"id":"999000111","peer":["111222333","agente-mario"],"session_id":7,"type":0,"uuid":"` + uuidSalvato +
		`","lasciapassare":"nonce-terzo","amministratore":false}`)
	manda(`{"id":"999000111","note":"#83100","session_id":7}`)
	manda(`{"action":"close","conn_id":7,"id":"999000111","session_id":7,"uuid":"` + uuidSalvato + `"}`)
	conn(8, "nonce-giusto", "")
	conn(9, "nonce-altro-pc", "")
	conn(10, "nonce-sconosciuto", "")
	conn(11, "", "")
	conn(12, "nonce-secondo", "true")
	conn(13, "", "true")
	// La 14 dice amministratore col new, e l'autorizzazione non lo ripete.
	manda(`{"action":"new","conn_id":14,"id":"999000111","ip":"192.0.2.10","session_id":14,"uuid":"` + uuidSalvato + `","amministratore":true}`)
	manda(`{"conn_id":14,"id":"999000111","peer":["111222333","mario"],"session_id":14,"type":0,"uuid":"` + uuidSalvato + `"}`)

	type verificata struct {
		ConnId                                    int64
		AgenteLasciapassare, TecnicoLasciapassare string
		Amministratore                            bool
	}
	var righe []verificata
	if err := service.DB.Raw("SELECT conn_id, agente_lasciapassare, tecnico_lasciapassare, amministratore FROM audit_conns ORDER BY id").Scan(&righe).Error; err != nil {
		t.Fatal(err)
	}
	want := []verificata{{7, "agente-mario", "mario", false}, {8, "", "", false}, {9, "", "", false}, {10, "", "", false},
		{11, "", "", false}, {12, "agente-mario", "mario", true}, {13, "", "", true}, {14, "", "", true}}
	if !reflect.DeepEqual(righe, want) {
		t.Errorf("audit_conns:\n got  %v\n want %v", righe, want)
	}
	var lps []model.Lasciapassare
	if err := service.DB.Order("id").Find(&lps).Error; err != nil {
		t.Fatal(err)
	}
	legati := map[string]int64{}
	for _, lp := range lps {
		legati[lp.Nonce] = lp.ConnId
	}
	if want := map[string]int64{"nonce-giusto": 7, "nonce-altro-pc": 0, "nonce-secondo": 12, "nonce-terzo": 0}; !reflect.DeepEqual(legati, want) {
		t.Errorf("conn_id dei lasciapassare: %v, attesi %v", legati, want)
	}
	log := registro.String()
	for motivo, n := range map[string]int{"connessione 7 del PC 999000111: lasciapassare": 0,
		"connessione 8 del PC 999000111: lasciapassare gia' usato":       2,
		"connessione 9 del PC 999000111: lasciapassare di un altro PC":   2,
		"connessione 10 del PC 999000111: nonce di nessun lasciapassare": 2} {
		if got := logger.Conta(log, "WARN", motivo); got != n {
			t.Errorf("%d warn %q, attesi %d", got, motivo, n)
		}
	}
	if logger.Conta(log, "INFO", "connessione 7 del PC 999000111 aperta col lasciapassare", "agente-mario") != 1 || strings.Contains(log, "nonce-") {
		t.Errorf("log:\n%s", log)
	}

	var coda []model.Evento
	if err := service.DB.Order("id").Find(&coda).Error; err != nil {
		t.Fatal(err)
	}
	agenti, ammin := map[string][]any{}, map[string][]any{}
	for _, e := range coda {
		var c map[string]any
		if err := json.Unmarshal([]byte(e.Corpo), &c); err != nil {
			t.Fatal(err)
		}
		agenti[e.Tipo] = append(agenti[e.Tipo], c["agente"])
		ammin[e.Tipo] = append(ammin[e.Tipo], c["sessione"].(map[string]any)["amministratore"])
	}
	utente := map[string]any{"agente": "agente-mario", "tecnico": "mario", "amministratore": false}
	admin := map[string]any{"agente": "agente-mario", "tecnico": "mario", "amministratore": true}
	wantAgenti := map[string][]any{
		service.EventoConnessioneAperta: {utente, nil, nil, nil, nil, admin, nil, nil},
		service.EventoConnessioneNota:   {nil},
		service.EventoConnessioneChiusa: {utente},
	}
	if !reflect.DeepEqual(agenti, wantAgenti) {
		t.Errorf("campo agente degli eventi:\n got  %v\n want %v", agenti, wantAgenti)
	}
	wantAmmin := map[string][]any{
		service.EventoConnessioneAperta: {false, false, false, false, false, true, true, true},
		service.EventoConnessioneNota:   {nil},
		service.EventoConnessioneChiusa: {false},
	}
	if !reflect.DeepEqual(ammin, wantAmmin) {
		t.Errorf("sessione.amministratore degli eventi:\n got  %v\n want %v", ammin, wantAmmin)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
