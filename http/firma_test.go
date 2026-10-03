package http

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// Vettore di prova della firma del dispositivo (README, "Firma del
// dispositivo"): il client firma questa richiesta con la chiave dal seed
// 00 01 02 ... 1f e deve ottenere esattamente vettoreFirma.
const (
	vettoreMetodo   = "POST"
	vettorePercorso = "/api/heartbeat"
	vettoreTs       = "1791000000"
	vettoreCorpo    = `{"id":"999000111","uuid":"dXVpZA=="}`
	vettoreSha      = "8ce852e69dbc62f20c29f8fcfb77e1a1922d73c7a185de173bb31139f4089d10"
	vettorePk       = "A6EHv/POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg="
	vettoreFirma    = "otUjpW4BdAQnl0TuN1E/GmD3LAx38zzpYTczckv70y3hYSIVtkA7urCTqftTR9BXPMoDc//+nyxItL73nbrHCw=="
)

// TestFirmaVettore fissa il formato: messaggio, chiave pubblica e firma del
// vettore di prova, costruiti a mano come dice il README.
func TestFirmaVettore(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	k := ed25519.NewKeyFromSeed(seed)
	messaggio := "remotek-api-v1\n" + vettoreMetodo + "\n" + vettorePercorso + "\n" + vettoreTs + "\n" + vettoreSha
	if got := string(service.MessaggioFirma(vettoreMetodo, vettorePercorso, vettoreTs, []byte(vettoreCorpo))); got != messaggio {
		t.Errorf("MessaggioFirma:\n got  %q\n want %q", got, messaggio)
	}
	if got := pkDi(k); got != vettorePk {
		t.Errorf("pk dal seed: %s, attesa %s", got, vettorePk)
	}
	if got := base64.StdEncoding.EncodeToString(ed25519.Sign(k, []byte(messaggio))); got != vettoreFirma {
		t.Errorf("firma del vettore: %s, attesa %s", got, vettoreFirma)
	}
}

func pkDi(k ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))
}

func nuovaChiave(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, k, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// firmaPc restituisce X-Remotek-Firma per POST rotta con corpo e ts, costruita
// dal formato del README e non con MessaggioFirma.
func firmaPc(k ed25519.PrivateKey, rotta, corpo string, ts int64) string {
	h := sha256.Sum256([]byte(corpo))
	tsTesto := strconv.FormatInt(ts, 10)
	m := strings.Join([]string{"remotek-api-v1", "POST", rotta, tsTesto, hex.EncodeToString(h[:])}, "\n")
	return tsTesto + "." + base64.StdEncoding.EncodeToString(ed25519.Sign(k, []byte(m)))
}

// firmata manda corpo in POST a rotta con l'intestazione della firma, se
// non vuota.
func firmata(g *gin.Engine, rotta, corpo, intestazione string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", rotta, strings.NewReader(corpo))
	req.Header.Set("Content-Type", "application/json")
	if intestazione != "" {
		req.Header.Set(service.IntestazioneFirma, intestazione)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// chiaveSalvata legge con Raw la chiave di 999000111.
func chiaveSalvata(t *testing.T) string {
	t.Helper()
	var c []string
	if err := service.DB.Raw("SELECT chiave_pubblica FROM peers WHERE id = ?", "999000111").Scan(&c).Error; err != nil || len(c) != 1 {
		t.Fatalf("chiave di 999000111: %v %v", c, err)
	}
	return c[0]
}

func sysinfoConPk(uuid, nome, pk string) string {
	return strings.TrimSuffix(corpoSysinfo(uuid, nome), "}") + `,"pk":"` + pk + `"}`
}

const (
	corpoHeartbeat = `{"id":"999000111","modified_at":0,"uuid":"` + uuidSalvato + `","ver":1004090}`
	corpoConn      = `{"action":"new","conn_id":7,"id":"999000111","ip":"192.0.2.10","session_id":1,"uuid":"` + uuidSalvato + `"}`
	successoAudit  = `{"code":0,"message":"success","data":""}`
	mismatch       = `{"error":"Il dispositivo non corrisponde a quello registrato."}` //nolint:misspell // testo italiano del messaggio DeviceMismatch
)

// TestFirmaRegistrazione prova sul router vero che un sysinfo firmato con la
// sua pk registra la chiave di un PC nuovo, che da li' le richieste firmate
// giuste di sysinfo, heartbeat e audit/conn passano, e che Update, usato da
// sysinfo e heartbeat, non tocca la chiave.
func TestFirmaRegistrazione(t *testing.T) {
	g, registro := dispositiviDiProva(t)
	k := nuovaChiave(t)
	ora := time.Now().Unix()

	corpo := sysinfoConPk(uuidSalvato, "PC-VERO", pkDi(k))
	if rec := firmata(g, "/api/sysinfo", corpo, firmaPc(k, "/api/sysinfo", corpo, ora)); rec.Body.String() != "SYSINFO_UPDATED" {
		t.Fatalf("sysinfo firmato di un PC nuovo: %d %s", rec.Code, rec.Body)
	}
	if got := chiaveSalvata(t); got != pkDi(k) {
		t.Fatalf("chiave dopo il primo sysinfo firmato: %q, attesa la pk", got)
	}
	corpo = sysinfoConPk(uuidSalvato, "PC-NUOVO-NOME", pkDi(k))
	if rec := firmata(g, "/api/sysinfo", corpo, firmaPc(k, "/api/sysinfo", corpo, ora+1)); rec.Body.String() != "SYSINFO_UPDATED" || scheda(t)[0].Hostname != "PC-NUOVO-NOME" {
		t.Errorf("sysinfo firmato: %d %s, nome %s", rec.Code, rec.Body, scheda(t)[0].Hostname)
	}
	if rec := firmata(g, "/api/heartbeat", corpoHeartbeat, firmaPc(k, "/api/heartbeat", corpoHeartbeat, ora)); rec.Body.String() != "{}" || scheda(t)[0].LastOnlineTime == 0 {
		t.Errorf("heartbeat firmato: %d %s, ultimo contatto %d", rec.Code, rec.Body, scheda(t)[0].LastOnlineTime)
	}
	if rec := firmata(g, "/api/audit/conn", corpoConn, firmaPc(k, "/api/audit/conn", corpoConn, ora)); rec.Body.String() != successoAudit || righe(t, "audit_conns") != 1 {
		t.Errorf("audit/conn firmato: %d %s, %d righe", rec.Code, rec.Body, righe(t, "audit_conns"))
	}
	pe, err := service.AllService.PeerService.FindById("999000111")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllService.PeerService.Update(&model.Peer{RowId: pe.RowId, Hostname: "ALTRO"}); err != nil {
		t.Fatal(err)
	}
	if got := chiaveSalvata(t); got != pkDi(k) {
		t.Errorf("chiave dopo Update senza chiave: %q, attesa la pk", got)
	}
	if strings.Contains(registro.String(), "WARN") || strings.Contains(registro.String(), "ERROR") {
		t.Errorf("richieste firmate giuste, nel log:\n%s", registro)
	}
}

// TestFirmaRifiuti prova sul router vero che un PC con la chiave rifiuta
// la richiesta senza firma, con la firma di un'altra chiave, fuori finestra
// o ripetuta, con la risposta di oggi per la rotta e un warn con rotta, ID e
// motivo, senza uuid, chiave ne' firma.
func TestFirmaRifiuti(t *testing.T) {
	for _, tc := range []struct{ rotta, corpo, risposta string }{
		{"/api/sysinfo", corpoSysinfo(uuidSalvato, "INTRUSO"), mismatch},
		{"/api/heartbeat", corpoHeartbeat, "{}"},
		{"/api/audit/conn", corpoConn, successoAudit},
	} {
		k, altra := nuovaChiave(t), nuovaChiave(t)
		// Le firme si fanno quando parte la richiesta: fuori finestra di
		// 310 secondi, perche' il test non dipenda da quanto dura.
		for _, caso := range []struct {
			motivo string
			firma  func(ora int64) string
		}{
			{service.FirmaAssente, func(int64) string { return "" }},
			{service.FirmaNonValida, func(ora int64) string { return firmaPc(altra, tc.rotta, tc.corpo, ora) }},
			{service.FirmaNonValida, func(ora int64) string { return firmaPc(k, tc.rotta, tc.corpo+" ", ora) }},
			{service.FirmaFuoriTempo, func(ora int64) string { return firmaPc(k, tc.rotta, tc.corpo, ora-310) }},
			{service.FirmaFuoriTempo, func(ora int64) string { return firmaPc(k, tc.rotta, tc.corpo, ora+310) }},
			{service.FirmaRipetuta, func(ora int64) string { return firmaPc(k, tc.rotta, tc.corpo, ora) }},
		} {
			t.Run(tc.rotta+" "+caso.motivo, func(t *testing.T) {
				intestazione := caso.firma(time.Now().Unix())
				vero := &model.Peer{Id: "999000111", Hostname: "PC-VERO", Uuid: uuidSalvato, ChiavePubblica: pkDi(k), LastOnlineTime: 1}
				g, registro := dispositiviDiProva(t, vero)
				if caso.motivo == service.FirmaRipetuta {
					firmata(g, tc.rotta, tc.corpo, intestazione)
					registro.Reset()
				}
				prima, conn := scheda(t), righe(t, "audit_conns")
				rec := firmata(g, tc.rotta, tc.corpo, intestazione)
				if rec.Body.String() != tc.risposta {
					t.Errorf("risposta %d %s, attesa %s", rec.Code, rec.Body, tc.risposta)
				}
				if caso.motivo != service.FirmaRipetuta && (scheda(t)[0] != prima[0] || righe(t, "audit_conns") != conn) {
					t.Errorf("scheda %+v -> %+v, righe dell'audit %d -> %d", prima[0], scheda(t)[0], conn, righe(t, "audit_conns"))
				}
				senzaUuid(t, registro, tc.rotta)
				nelLog := registro.String()
				if logger.Conta(nelLog, "WARN", tc.rotta, "999000111", caso.motivo) != 1 {
					t.Errorf("nel log manca il warn con il motivo %q:\n%s", caso.motivo, nelLog)
				}
				_, sig, _ := strings.Cut(intestazione, ".")
				if strings.Contains(nelLog, pkDi(k)) || (sig != "" && strings.Contains(nelLog, sig)) {
					t.Errorf("nel log c'e' la chiave o la firma:\n%s", nelLog)
				}
			})
		}
	}
}

// TestFirmaSenzaChiave prova sul router vero che un PC senza chiave resta
// con le regole di ADR-0019: senza firma passa; un sysinfo con pk registra
// la chiave solo con firma valida e uuid giusto.
func TestFirmaSenzaChiave(t *testing.T) {
	k := nuovaChiave(t)
	ora := time.Now().Unix()
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato})
	if rec := firmata(g, "/api/heartbeat", corpoHeartbeat, ""); rec.Body.String() != "{}" || scheda(t)[0].LastOnlineTime == 0 {
		t.Errorf("heartbeat senza firma: %d %s, ultimo contatto %d", rec.Code, rec.Body, scheda(t)[0].LastOnlineTime)
	}
	if rec := firmata(g, "/api/audit/conn", corpoConn, ""); rec.Body.String() != successoAudit || righe(t, "audit_conns") != 1 {
		t.Errorf("audit/conn senza firma: %d %s, %d righe", rec.Code, rec.Body, righe(t, "audit_conns"))
	}
	for _, caso := range []struct {
		nome, corpo, intestazione, risposta string
	}{
		{"senza firma", sysinfoConPk(uuidSalvato, "A", pkDi(k)), "", "SYSINFO_UPDATED"},
		{"firma di un'altra chiave", sysinfoConPk(uuidSalvato, "B", pkDi(k)), "altra", "SYSINFO_UPDATED"},
		{"uuid diverso", sysinfoConPk(uuidAltro, "C", pkDi(k)), "giusta", mismatch},
	} {
		switch caso.intestazione {
		case "altra":
			caso.intestazione = firmaPc(nuovaChiave(t), "/api/sysinfo", caso.corpo, ora)
		case "giusta":
			caso.intestazione = firmaPc(k, "/api/sysinfo", caso.corpo, ora)
		}
		if rec := firmata(g, "/api/sysinfo", caso.corpo, caso.intestazione); rec.Body.String() != caso.risposta {
			t.Errorf("sysinfo %s: %d %s, atteso %s", caso.nome, rec.Code, rec.Body, caso.risposta)
		}
		if got := chiaveSalvata(t); got != "" {
			t.Errorf("sysinfo %s: chiave registrata %q", caso.nome, got)
		}
	}
	if logger.Conta(registro.String(), "WARN", "/api/sysinfo", "999000111", "chiave non registrata: "+service.FirmaNonValida) != 1 {
		t.Errorf("nel log manca il warn della chiave non registrata:\n%s", registro)
	}
}

// TestFirmaChiaveDiversa prova sul router vero che un sysinfo con uuid
// giusto, firmato con una pk diversa da quella salvata, non cambia niente,
// risponde come DeviceMismatch e scrive un error con rotta e ID.
func TestFirmaChiaveDiversa(t *testing.T) {
	k, altra := nuovaChiave(t), nuovaChiave(t)
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Hostname: "PC-VERO", Uuid: uuidSalvato, ChiavePubblica: pkDi(k)})
	prima := scheda(t)
	corpo := sysinfoConPk(uuidSalvato, "REINSTALLATO", pkDi(altra))
	if rec := firmata(g, "/api/sysinfo", corpo, firmaPc(altra, "/api/sysinfo", corpo, time.Now().Unix())); rec.Body.String() != mismatch {
		t.Errorf("sysinfo con pk diversa: %d %s, atteso %s", rec.Code, rec.Body, mismatch)
	}
	if got := chiaveSalvata(t); got != pkDi(k) || scheda(t)[0] != prima[0] {
		t.Errorf("sysinfo con pk diversa: chiave %q, scheda %+v", got, scheda(t)[0])
	}
	nelLog := registro.String()
	if logger.Conta(nelLog, "ERROR", "POST /api/sysinfo: ", "999000111") != 1 || strings.Contains(nelLog, "WARN") {
		t.Errorf("nel log serve un error con rotta e ID, e nessun warn:\n%s", nelLog)
	}
}

// TestFirmaObbligatoria prova sul router vero app.firma-obbligatoria: un PC
// senza chiave e senza firma e' rifiutato; un sysinfo firmato con pk e uuid
// giusto registra la chiave.
func TestFirmaObbligatoria(t *testing.T) {
	g, registro := dispositiviDiProva(t, &model.Peer{Id: "999000111", Hostname: "PC-VERO", Uuid: uuidSalvato})
	global.Config.App.FirmaObbligatoria = true
	t.Cleanup(func() { global.Config.App.FirmaObbligatoria = false })
	k := nuovaChiave(t)
	prima := scheda(t)
	if rec := firmata(g, "/api/heartbeat", corpoHeartbeat, ""); rec.Body.String() != "{}" || scheda(t)[0] != prima[0] {
		t.Errorf("heartbeat senza firma: %d %s, scheda %+v", rec.Code, rec.Body, scheda(t)[0])
	}
	if rec := firmata(g, "/api/audit/conn", corpoConn, firmaPc(k, "/api/audit/conn", corpoConn, time.Now().Unix())); rec.Body.String() != successoAudit || righe(t, "audit_conns") != 0 {
		t.Errorf("audit/conn firmato senza chiave registrata: %d %s, %d righe", rec.Code, rec.Body, righe(t, "audit_conns"))
	}
	if rec := firmata(g, "/api/sysinfo", sysinfoConPk(uuidSalvato, "X", pkDi(k)), ""); rec.Body.String() != mismatch || scheda(t)[0] != prima[0] {
		t.Errorf("sysinfo senza firma: %d %s, scheda %+v", rec.Code, rec.Body, scheda(t)[0])
	}
	nelLog := registro.String()
	for _, m := range []struct{ rotta, motivo string }{
		{"/api/heartbeat", service.FirmaAssente}, {"/api/audit/conn", service.SenzaChiave}, {"/api/sysinfo", service.FirmaAssente},
	} {
		if logger.Conta(nelLog, "WARN", m.rotta, "999000111", m.motivo) != 1 {
			t.Errorf("POST %s: nel log manca il warn %q:\n%s", m.rotta, m.motivo, nelLog)
		}
	}
	corpo := sysinfoConPk(uuidSalvato, "PC-FIRMATO", pkDi(k))
	if rec := firmata(g, "/api/sysinfo", corpo, firmaPc(k, "/api/sysinfo", corpo, time.Now().Unix())); rec.Body.String() != "SYSINFO_UPDATED" || chiaveSalvata(t) != pkDi(k) {
		t.Errorf("sysinfo firmato con pk: %d %s, chiave %q", rec.Code, rec.Body, chiaveSalvata(t))
	}
}
