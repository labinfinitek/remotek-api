package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/config"
	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/model/custom_types"
)

const segretoDiProva = "segreto-di-prova-di-almeno-32-caratteri"

// TestFirmaVettore fissa il vettore del README: chi riceve lo usa per
// provare la sua verifica.
func TestFirmaVettore(t *testing.T) {
	got := Firma(segretoDiProva, 1791000000, []byte(`{"schema":1,"tipo":"prova"}`))
	want := "t=1791000000,v1=47977e5349c23bd14a1a9d3200bed968a3092152ca7f73b73c6581a5f0b47013"
	if got != want {
		t.Errorf("Firma:\n got  %s\n want %s", got, want)
	}
}

// ricevuto e' una richiesta arrivata al ricevitore di prova.
type ricevuto struct {
	id, firma, corpo string
}

// ricevitore e' l'n8n di prova: risponde con gli stati di risposte, poi
// sempre con l'ultimo, e ricorda le richieste.
type ricevitore struct {
	mu       sync.Mutex
	risposte []int
	arrivati []ricevuto
}

func (r *ricevitore) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	corpo, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.arrivati = append(r.arrivati, ricevuto{req.Header.Get("X-Remotek-Evento-Id"), req.Header.Get("X-Remotek-Evento-Firma"), string(corpo)})
	stato := r.risposte[min(len(r.arrivati), len(r.risposte))-1]
	w.WriteHeader(stato)
}

// codaDiProva prepara database, log e configurazione dei servizi con
// l'integrazione verso un ricevitore che risponde con risposte, e una
// consegna con l'orologio fermo all'ora restituita, che il test sposta.
func codaDiProva(t *testing.T, risposte ...int) (*Consegna, *ricevitore, *time.Time, *strings.Builder) {
	t.Helper()
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.Evento{}, &model.Peer{}, &model.AuditConn{})
	}
	if err != nil {
		t.Fatal(err)
	}
	r := &ricevitore{risposte: risposte}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	registro := &strings.Builder{}
	precDB, precConfig, precLogger, precServizi := DB, Config, Logger, AllService
	t.Cleanup(func() { DB, Config, Logger, AllService = precDB, precConfig, precLogger, precServizi })
	DB, Logger, AllService = db, applog.Su(registro), new(Service)
	Config = &config.Config{Integrazione: config.Integrazione{Url: srv.URL, Segreto: segretoDiProva}}
	adesso := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	k := NuovaConsegna(Config.Integrazione)
	k.Ora = func() time.Time { return adesso }
	return k, r, &adesso, registro
}

// eventi restituisce la coda in ordine di id.
func eventi(t *testing.T) []model.Evento {
	t.Helper()
	var res []model.Evento
	if err := DB.Order("id").Find(&res).Error; err != nil {
		t.Fatal(err)
	}
	return res
}

// passo fa un Passo e controlla se ha mandato un evento.
func passo(t *testing.T, k *Consegna, mandato bool) {
	t.Helper()
	got, err := k.Passo(t.Context())
	if err != nil || got != mandato {
		t.Fatalf("Passo: %t, %v; atteso %t, nil", got, err, mandato)
	}
}

// TestConsegnaRiuscita: l'evento arriva al primo colpo col corpo salvato,
// l'id negli header e la firma che chi riceve ricalcola; poi e'
// consegnato e non si manda piu'.
func TestConsegnaRiuscita(t *testing.T) {
	k, r, adesso, registro := codaDiProva(t, 204)
	if err := accoda(corpoEvento{Tipo: "prova", Pc: PcEvento{Id: "999000111"}}, *adesso); err != nil {
		t.Fatal(err)
	}
	passo(t, k, true)
	passo(t, k, false)
	e := eventi(t)[0]
	if len(r.arrivati) != 1 {
		t.Fatalf("%d richieste, attesa 1", len(r.arrivati))
	}
	a := r.arrivati[0]
	ts := strconv.FormatInt(adesso.Unix(), 10)
	if a.corpo != e.Corpo || a.id != e.EventoId || a.firma != Firma(segretoDiProva, adesso.Unix(), []byte(e.Corpo)) || !strings.HasPrefix(a.firma, "t="+ts+",v1=") {
		t.Errorf("arrivato %+v, evento %+v", a, e)
	}
	if e.Esito != model.EventoConsegnato || e.Tentativi != 1 || e.Chiuso != adesso.Unix() {
		t.Errorf("evento dopo la consegna: %+v", e)
	}
	if registro.Len() != 0 {
		t.Errorf("log non vuoto:\n%s", registro)
	}
}

// TestConsegnaRipetuta: dopo un 500 l'evento si riprova dopo 30 secondi,
// non prima, con lo stesso id; dopo il 200 non si manda piu'. Un redirect
// non si segue e vale come non consegnato.
func TestConsegnaRipetuta(t *testing.T) {
	for _, primo := range []int{500, 302} {
		t.Run(strconv.Itoa(primo), func(t *testing.T) {
			k, r, adesso, registro := codaDiProva(t, primo, 200)
			if err := accoda(corpoEvento{Tipo: "prova", Pc: PcEvento{Id: "999000111"}}, *adesso); err != nil {
				t.Fatal(err)
			}
			passo(t, k, true)
			*adesso = adesso.Add(29 * time.Second)
			passo(t, k, false)
			*adesso = adesso.Add(time.Second)
			passo(t, k, true)
			*adesso = adesso.Add(time.Hour)
			passo(t, k, false)
			if len(r.arrivati) != 2 || r.arrivati[0].id != r.arrivati[1].id || r.arrivati[0].corpo != r.arrivati[1].corpo {
				t.Errorf("richieste: %+v, attese due uguali", r.arrivati)
			}
			if e := eventi(t)[0]; e.Esito != model.EventoConsegnato || e.Tentativi != 2 {
				t.Errorf("evento: %+v", e)
			}
			if applog.Conta(registro.String(), "WARN", "stato "+strconv.Itoa(primo)) != 1 || strings.Contains(registro.String(), "127.0.0.1") {
				t.Errorf("atteso un warn con lo stato e senza l'URL:\n%s", registro)
			}
		})
	}
}

// TestConsegnaFallita: un ricevitore che risponde sempre 500 riceve
// l'evento con attese da 30 secondi che raddoppiano fino a un'ora, per 24
// ore; poi una sola riga ERROR "evento non consegnato" con tipo e id, e
// l'evento resta fallito.
func TestConsegnaFallita(t *testing.T) {
	k, r, adesso, registro := codaDiProva(t, 500)
	inizio := *adesso
	if err := accoda(corpoEvento{Tipo: EventoConnessioneChiusa, Pc: PcEvento{Id: "999000111"}}, inizio); err != nil {
		t.Fatal(err)
	}
	var attese []time.Duration
	prec := inizio
	for range 100 {
		passo(t, k, true)
		e := eventi(t)[0]
		if e.Esito != model.EventoInAttesa {
			break
		}
		prossimo := time.Unix(e.Prossimo, 0)
		attese = append(attese, prossimo.Sub(prec))
		prec, *adesso = prossimo, prossimo
	}
	e := eventi(t)[0]
	if e.Esito != model.EventoFallito || adesso.Sub(inizio) != 24*time.Hour || e.Tentativi != len(r.arrivati) {
		t.Fatalf("dopo %s e %d richieste: %+v", adesso.Sub(inizio), len(r.arrivati), e)
	}
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 32 * time.Minute, time.Hour, time.Hour}
	if !slices.Equal(attese[:len(want)], want) {
		t.Errorf("attese %v, le prime attese %v", attese, want)
	}
	if n := applog.Conta(registro.String(), "ERROR", "evento non consegnato", EventoConnessioneChiusa, e.EventoId); n != 1 {
		t.Errorf("%d righe ERROR, attesa 1:\n%s", n, registro)
	}
	*adesso = adesso.Add(time.Hour)
	passo(t, k, false)
}

// TestOrdineCoda: due eventi pronti partono in ordine di creazione; il
// primo riceve un 500 e il secondo si consegna al passo dopo, prima che il
// primo si riprovi: un evento che si riprova non blocca la coda.
func TestOrdineCoda(t *testing.T) {
	k, r, adesso, _ := codaDiProva(t, 500, 200)
	var ids []string
	for range 2 {
		if err := accoda(corpoEvento{Tipo: "prova", Pc: PcEvento{Id: "999000111"}}, *adesso); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range eventi(t) {
		ids = append(ids, e.EventoId)
	}
	passo(t, k, true)
	passo(t, k, true)
	passo(t, k, false)
	if len(r.arrivati) != 2 || r.arrivati[0].id != ids[0] || r.arrivati[1].id != ids[1] {
		t.Fatalf("arrivati %+v, attesi %v in quest'ordine", r.arrivati, ids)
	}
	coda := eventi(t)
	if coda[0].Esito != model.EventoInAttesa || coda[1].Esito != model.EventoConsegnato {
		t.Errorf("esiti %q e %q, attesi in attesa e consegnato", coda[0].Esito, coda[1].Esito)
	}
	*adesso = adesso.Add(30 * time.Second)
	passo(t, k, true)
	if len(r.arrivati) != 3 || r.arrivati[2].id != ids[0] || eventi(t)[0].Esito != model.EventoConsegnato {
		t.Errorf("il primo evento non e' stato riprovato e consegnato: %+v", r.arrivati)
	}
}

// TestPuliziaCoda: consegnati e falliti si cancellano 7 giorni dopo la
// fine; quelli in attesa restano.
func TestPuliziaCoda(t *testing.T) {
	k, _, adesso, _ := codaDiProva(t, 200)
	vecchio, recente := adesso.Add(-7*24*time.Hour-time.Second).Unix(), adesso.Add(-6*24*time.Hour).Unix()
	for i, e := range []model.Evento{
		{Esito: model.EventoConsegnato, Chiuso: vecchio},
		{Esito: model.EventoFallito, Chiuso: vecchio},
		{Esito: model.EventoConsegnato, Chiuso: recente},
		{Esito: model.EventoInAttesa, Prossimo: adesso.Add(time.Hour).Unix(), Creato: vecchio},
	} {
		e.EventoId, e.Tipo, e.Corpo = strconv.Itoa(i), "prova", "{}"
		if err := DB.Create(&e).Error; err != nil {
			t.Fatal(err)
		}
	}
	passo(t, k, false)
	var resto []string
	for _, e := range eventi(t) {
		resto = append(resto, e.EventoId)
	}
	if !slices.Equal(resto, []string{"2", "3"}) {
		t.Errorf("dopo la pulizia restano %v, attesi [2 3]", resto)
	}
}

// TestCampiEventi controlla l'elenco dei campi di ogni tipo e che uuid,
// IP, guid, session_id e chiave non escano. La connessione e' quella dopo
// l'autorizzazione, che porta tecnico e type: l'apertura esce li'.
func TestCampiEventi(t *testing.T) {
	codaDiProva(t, 200)
	pc := &model.Peer{Id: "999000111", Hostname: "PC-ROSSI", Alias: "Rossi", Uuid: "dXVpZC1zZWdyZXRv", ChiavePubblica: "Y2hpYXZl", LastOnlineIp: "198.51.100.1"}
	if err := DB.Create(pc).Error; err != nil {
		t.Fatal(err)
	}
	inizio := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	conn := &model.AuditConn{ConnId: 7, PeerId: "999000111", FromName: "mario", Type: 4, Ip: "203.0.113.7", Uuid: "dXVpZC1zZWdyZXRv",
		SessionId: "9007199254740993", Guid: "0123456789abcdef", Note: "#83100", TimeModel: model.TimeModel{CreatedAt: custom_types.AutoTime(inizio)}}
	chiusa := *conn
	chiusa.CloseTime = inizio.Add(90 * time.Second).Unix()
	for _, tc := range []struct {
		tipo string
		conn *model.AuditConn
	}{{EventoConnessioneAperta, conn}, {EventoConnessioneNota, conn}, {EventoConnessioneChiusa, &chiusa}} {
		if err := AllService.EventiService.AccodaSessione(tc.tipo, tc.conn); err != nil {
			t.Fatal(err)
		}
	}
	campi := []string{"id", "ora", "pc", "schema", "sessione", "tipo"}
	campiPc := []string{"alias", "id", "nome"}
	campiSessione := []string{"conn_id", "durata_s", "fine", "inizio", "nota", "tecnico", "tipo"}
	for i, e := range eventi(t) {
		var c map[string]any
		if err := json.Unmarshal([]byte(e.Corpo), &c); err != nil {
			t.Fatal(err)
		}
		s := c["sessione"].(map[string]any)
		if !slices.Equal(chiavi(c), campi) || !slices.Equal(chiavi(c["pc"].(map[string]any)), campiPc) || !slices.Equal(chiavi(s), campiSessione) {
			t.Errorf("%s: campi %v, pc %v, sessione %v", e.Tipo, chiavi(c), chiavi(c["pc"].(map[string]any)), chiavi(s))
		}
		for _, vietato := range []string{"dXVpZC1zZWdyZXRv", "Y2hpYXZl", "198.51.100.1", "203.0.113.7", "0123456789abcdef", "9007199254740993"} {
			if strings.Contains(e.Corpo, vietato) {
				t.Errorf("%s: nel corpo c'e' %s: %s", e.Tipo, vietato, e.Corpo)
			}
		}
		want := map[string]any{"schema": 1.0, "id": e.EventoId, "tipo": e.Tipo, "ora": c["ora"],
			"pc":       map[string]any{"id": "999000111", "nome": "PC-ROSSI", "alias": "Rossi"},
			"sessione": map[string]any{"conn_id": 7.0, "tipo": 4.0, "tecnico": "mario", "inizio": "2026-10-11T09:00:00Z", "fine": nil, "durata_s": nil, "nota": "#83100"}}
		if i == 2 {
			want["sessione"].(map[string]any)["fine"], want["sessione"].(map[string]any)["durata_s"] = "2026-10-11T09:01:30Z", 90.0
		}
		if !reflect.DeepEqual(c, want) {
			t.Errorf("%s:\n got  %v\n want %v", e.Tipo, c, want)
		}
		if _, err := time.Parse(time.RFC3339, c["ora"].(string)); err != nil || !strings.HasSuffix(c["ora"].(string), "Z") {
			t.Errorf("%s: ora %v non RFC 3339 UTC", e.Tipo, c["ora"])
		}
	}
}

// chiavi restituisce le chiavi di m in ordine.
func chiavi(m map[string]any) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	slices.Sort(res)
	return res
}

// TestSenzaUrlNienteInCoda: con integrazione.url vuoto AccodaSessione non
// scrive niente, nemmeno senza la tabella, e la consegna non parte.
func TestSenzaUrlNienteInCoda(t *testing.T) {
	codaDiProva(t, 200)
	Config.Integrazione = config.Integrazione{}
	if err := DB.Migrator().DropTable(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	if err := AllService.EventiService.AccodaSessione(EventoConnessioneAperta, &model.AuditConn{PeerId: "999000111"}); err != nil {
		t.Errorf("AccodaSessione senza URL: %v", err)
	}
	AvviaConsegna(Config.Integrazione)()
}

// TestCampiAltriEventi controlla, per file.trasferiti, terminale.chiuso,
// pc.nuovo, pc.possibile_copia e agente.lasciapassare, il corpo intero e che nomi e percorsi dei
// file, indirizzi, uuid, chiave e guid non escano.
func TestCampiAltriEventi(t *testing.T) {
	codaDiProva(t, 200)
	pc := &model.Peer{Id: "999000111", Hostname: "PC-ROSSI", Alias: "Rossi", Uuid: "dXVpZC1zZWdyZXRv", ChiavePubblica: "Y2hpYXZl", LastOnlineIp: "198.51.100.1"}
	if err := DB.Create(pc).Error; err != nil {
		t.Fatal(err)
	}
	inizio := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	conn := &model.AuditConn{ConnId: 7, PeerId: "999000111", FromName: "mario", Type: model.AuditConnTerminale, Ip: "203.0.113.7",
		Uuid: "dXVpZC1zZWdyZXRv", SessionId: "9007199254740993", Guid: "0123456789abcdef", Note: "#83100",
		TimeModel: model.TimeModel{CreatedAt: custom_types.AutoTime(inizio)}}
	file := func(tipo, num int) *model.AuditFile {
		return &model.AuditFile{PeerId: "999000111", FromPeer: "999000222", FromName: "mario", Type: tipo, Num: num, IsFile: true,
			Path: `C:\Users\rossi\segreti.txt`, Info: `{"files":[["segreti.txt",12]],"ip":"203.0.113.7","name":"mario","num":1}`, Ip: "203.0.113.7", Uuid: "dXVpZC1zZWdyZXRv"}
	}
	es := AllService.EventiService
	for _, err := range []error{
		es.AccodaFile(file(0, 1)),
		es.AccodaFile(file(1, 3)),
		es.AccodaTerminale(conn, &model.AuditTerminalVerifica{Integra: true, Blocchi: 3, Fine: true, Hash: "ab12"}),
		es.AccodaPcNuovo(pc),
		es.AccodaCopia("999000111", 10),
		es.AccodaLasciapassare("999000111", &model.User{Username: "agente-mario", Password: "segreto"}, &model.User{Username: "mario"}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	pcJSON := map[string]any{"id": "999000111", "nome": "PC-ROSSI", "alias": "Rossi"}
	want := []map[string]any{
		{"tipo": EventoFileTrasferiti, "file": map[string]any{"direzione": "dal_pc", "numero": 1.0}},
		{"tipo": EventoFileTrasferiti, "file": map[string]any{"direzione": "al_pc", "numero": 3.0}},
		{"tipo": EventoTerminaleChiuso, "verifica": map[string]any{"integra": true, "hash": "ab12"},
			"sessione": map[string]any{"conn_id": 7.0, "tipo": 4.0, "tecnico": "mario", "inizio": "2026-10-11T09:00:00Z", "fine": nil, "durata_s": nil, "nota": "#83100"}},
		{"tipo": EventoPcNuovo},
		{"tipo": EventoPcCopia, "copia": map[string]any{"cambi": 10.0}},
		{"tipo": EventoAgenteLasciapassare, "agente": map[string]any{"agente": "agente-mario", "tecnico": "mario", "amministratore": true}},
	}
	coda := eventi(t)
	if len(coda) != len(want) {
		t.Fatalf("%d eventi, attesi %d", len(coda), len(want))
	}
	for i, e := range coda {
		var c map[string]any
		if err := json.Unmarshal([]byte(e.Corpo), &c); err != nil {
			t.Fatal(err)
		}
		w := want[i]
		w["schema"], w["id"], w["ora"], w["pc"] = 1.0, e.EventoId, c["ora"], pcJSON
		if !reflect.DeepEqual(c, w) || e.Tipo != w["tipo"] {
			t.Errorf("%s:\n got  %v\n want %v", e.Tipo, c, w)
		}
		for _, vietato := range []string{"segreti", `Users`, "999000222", "dXVpZC1zZWdyZXRv", "Y2hpYXZl", "198.51.100.1", "203.0.113.7", "0123456789abcdef", "9007199254740993", `"blocchi"`} {
			if strings.Contains(e.Corpo, vietato) {
				t.Errorf("%s: nel corpo c'e' %s: %s", e.Tipo, vietato, e.Corpo)
			}
		}
	}
}

// TestAccodaFileTypeSconosciuto: un type diverso da 0 e 1 non accoda
// niente e da' un warn con il type e l'ID del PC.
func TestAccodaFileTypeSconosciuto(t *testing.T) {
	_, _, _, registro := codaDiProva(t, 200)
	f := &model.AuditFile{PeerId: "999000111", Type: 2, Num: 1}
	if err := AllService.EventiService.AccodaFile(f); err != nil {
		t.Fatal(err)
	}
	if n := len(eventi(t)); n != 0 {
		t.Errorf("%d eventi in coda, attesi 0", n)
	}
	if n := applog.Conta(registro.String(), "WARN", "type 2", "999000111"); n != 1 {
		t.Errorf("%d warn col type e l'ID del PC, atteso 1:\n%s", n, registro)
	}
}

// TestGuastoCoda: con la coda guasta (tabella che non c'e') gira scrive una
// sola riga ERROR, per quanti giri faccia, e una INFO quando la coda torna.
func TestGuastoCoda(t *testing.T) {
	k, _, adesso, registro := codaDiProva(t, 200)
	if err := DB.Migrator().DropTable(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	var giri atomic.Int64
	k.intervallo = time.Millisecond
	k.Ora = func() time.Time { giri.Add(1); return *adesso }
	aspetta := func(n int64) {
		t.Helper()
		for scade := time.Now().Add(5 * time.Second); giri.Load() < n; time.Sleep(time.Millisecond) {
			if time.Now().After(scade) {
				t.Fatalf("%d giri, attesi %d", giri.Load(), n)
			}
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	fatto := make(chan struct{})
	go func() {
		defer close(fatto)
		k.gira(ctx)
	}()
	aspetta(5)
	if err := DB.AutoMigrate(&model.Evento{}); err != nil {
		t.Fatal(err)
	}
	aspetta(giri.Load() + 5)
	cancel()
	<-fatto
	testo := registro.String()
	if n := applog.Conta(testo, "ERROR", "coda degli eventi"); n != 1 {
		t.Errorf("%d righe ERROR, attesa 1:\n%s", n, testo)
	}
	if n := applog.Conta(testo, "INFO", "coda degli eventi"); n != 1 {
		t.Errorf("%d righe INFO, attesa 1:\n%s", n, testo)
	}
}
