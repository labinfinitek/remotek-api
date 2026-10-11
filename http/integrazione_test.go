package http

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/model/custom_types"
	"github.com/lejianwen/rustdesk-api/v2/service"
)

// integrazioneLettura prepara il router vero con l'utente del pannello
// (admin, token di login tokenDelPannello), due PC, tre connessioni (una
// terminale con la trascrizione) e i token di integrazione "n8n" valido,
// "vecchio" scaduto e "revocato"; restituisce router e token validi e non.
func integrazioneLettura(t *testing.T) (*gin.Engine, map[string]string, *strings.Builder) {
	t.Helper()
	g, _, registro := pannello(t, true)
	precJwt := global.Jwt
	global.Jwt = jwt.NewJwt("", time.Hour)
	t.Cleanup(func() { global.Jwt = precJwt })
	if err := service.DB.AutoMigrate(&model.Peer{}, &model.AuditConn{}, &model.AuditTerminal{}, &model.TokenIntegrazione{}); err != nil {
		t.Fatal(err)
	}
	crea(t, &model.Peer{Id: "999000111", Uuid: uuidSalvato, Hostname: "PC-ROSSI", Alias: "Rossi", Version: "1.4.9",
		LastOnlineTime: time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC).Unix(), LastOnlineIp: "198.51.100.1"})
	// La chiave sta sul PC senza trascrizione: con la chiave i blocchi
	// dovrebbero essere firmati.
	crea(t, &model.Peer{Id: "999000222", Uuid: uuidAltro, ChiavePubblica: "Y2hpYXZl"})
	ieri := custom_types.AutoTime(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))
	oggi := custom_types.AutoTime(time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC))
	crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 5, PeerId: "999000111", Ip: "203.0.113.7", Uuid: uuidSalvato, SessionId: "424242", Guid: "guidsegreto", TimeModel: model.TimeModel{CreatedAt: ieri}})
	crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 7, PeerId: "999000111", Type: model.AuditConnTerminale, FromName: "mario", Ip: "203.0.113.7", Note: "#83100", TimeModel: model.TimeModel{CreatedAt: oggi}})
	crea(t, &model.AuditConn{Action: model.AuditActionNew, ConnId: 9, PeerId: "999000222", TimeModel: model.TimeModel{CreatedAt: oggi}})
	manda(t, g, catena("whoami\n", "rossi\n")...)

	ts := service.AllService.TokenIntegrazioneService
	tok := map[string]string{"login": tokenDelPannello}
	adesso := time.Now()
	for nome, quando := range map[string]time.Time{"n8n": adesso, "vecchio": adesso.Add(-service.DurataTokenIntegrazione - time.Minute), "revocato": adesso} {
		k, err := ts.Crea(nome, quando)
		if err != nil {
			t.Fatal(err)
		}
		tok[nome] = k
	}
	if err := ts.Revoca("revocato", adesso); err != nil {
		t.Fatal(err)
	}
	return g, tok, registro
}

// conBearer manda una richiesta con Authorization: auth, se non e' vuoto.
func conBearer(g *gin.Engine, metodo, rotta, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, rotta, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	return rec
}

// TestIntegrazioneLettura: col token di integrazione le tre rotte
// rispondono in GET con i campi del README, senza uuid, IP, chiavi, guid o
// session_id; i filtri per PC e da quando funzionano; la verifica e' quella
// del pannello e una connessione non terminale da' 404.
func TestIntegrazioneLettura(t *testing.T) {
	g, tok, _ := integrazioneLettura(t)
	auth := "Bearer " + tok["n8n"]
	vietati := []string{uuidSalvato, uuidAltro, "198.51.100.1", "203.0.113.7", "Y2hpYXZl", "guidsegreto", "424242", "whoami", `"ip"`, `"uuid"`}
	leggi := func(rotta string, v any) string {
		t.Helper()
		rec := conBearer(g, "GET", rotta, auth)
		if rec.Code != 200 {
			t.Fatalf("GET %s: %d %s", rotta, rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatal(err)
		}
		for _, x := range vietati {
			if strings.Contains(rec.Body.String(), x) {
				t.Errorf("GET %s: nella risposta c'e' %s: %s", rotta, x, rec.Body)
			}
		}
		return rec.Body.String()
	}

	var pc map[string]any
	leggi("/api/integrazione/pc", &pc)
	elenco := pc["elenco"].([]any)
	primo := elenco[0].(map[string]any)
	if pc["totale"] != 2.0 || len(elenco) != 2 || !slices.Equal(chiaviDi(primo), []string{"alias", "id", "nome", "possibile_copia", "ultimo_contatto", "versione"}) || //nolint:misspell // italiano, nome deciso in ADR-0024
		primo["nome"] != "PC-ROSSI" || primo["ultimo_contatto"] != "2026-10-11T08:00:00Z" || elenco[1].(map[string]any)["ultimo_contatto"] != nil {
		t.Errorf("PC: %v", pc)
	}

	var conn struct {
		Totale int
		Elenco []map[string]any
	}
	leggi("/api/integrazione/connessioni", &conn)
	if conn.Totale != 3 {
		t.Errorf("connessioni: %d, attese 3", conn.Totale)
	}
	if !slices.Equal(chiaviDi(conn.Elenco[0]), []string{"conn_id", "durata_s", "fine", "id", "inizio", "nota", "pc", "tecnico", "tipo"}) {
		t.Errorf("campi della connessione: %v", chiaviDi(conn.Elenco[0]))
	}
	leggi("/api/integrazione/connessioni?pc=999000111&da=2026-10-11T00:00:00Z", &conn)
	if conn.Totale != 1 || conn.Elenco[0]["conn_id"] != 7.0 || conn.Elenco[0]["tecnico"] != "mario" || conn.Elenco[0]["nota"] != "#83100" {
		t.Errorf("connessioni filtrate: %+v", conn)
	}
	if rec := conBearer(g, "GET", "/api/integrazione/connessioni?da=ieri", auth); rec.Code != 400 {
		t.Errorf("da non valido: %d %s, atteso 400", rec.Code, rec.Body)
	}

	terminale := uint(conn.Elenco[0]["id"].(float64))
	var v model.AuditTerminalVerifica
	leggi(fmt.Sprint("/api/integrazione/connessioni/", terminale, "/verifica"), &v)
	if pannello := verifica(t, g, terminale); v != pannello || !v.Integra || v.Blocchi != 2 {
		t.Errorf("verifica %+v, quella del pannello %+v", v, pannello)
	}
	for _, rotta := range []string{"/api/integrazione/connessioni/1/verifica", "/api/integrazione/connessioni/99/verifica", "/api/integrazione/connessioni/x/verifica"} {
		if rec := conBearer(g, "GET", rotta, auth); rec.Code != 404 {
			t.Errorf("GET %s: %d %s, atteso 404", rotta, rec.Code, rec.Body)
		}
	}
}

// chiaviDi restituisce le chiavi di m in ordine.
func chiaviDi(m map[string]any) []string {
	var res []string
	for k := range m {
		res = append(res, k)
	}
	slices.Sort(res)
	return res
}

// TestIntegrazioneSoloToken: senza token, con un token scaduto, revocato,
// sconosciuto o di login le rotte rispondono 401; con un metodo diverso da
// GET non rispondono 2xx; il token di integrazione non apre il pannello ne'
// le rotte del client. Nel log non c'e' nessun token, e un token scaduto
// o revocato ha un warn col suo nome.
func TestIntegrazioneSoloToken(t *testing.T) {
	g, tok, registro := integrazioneLettura(t)
	const rotta = "/api/integrazione/pc"
	for nome, auth := range map[string]string{
		"senza": "", "bearer vuoto": "Bearer ", "basic": "Basic " + tok["n8n"], "senza bearer": tok["n8n"],
		"scaduto": "Bearer " + tok["vecchio"], "revocato": "Bearer " + tok["revocato"], "sconosciuto": "Bearer " + tok["n8n"] + "x",
		"login": "Bearer " + tok["login"],
	} {
		if rec := conBearer(g, "GET", rotta, auth); rec.Code != 401 || rec.Body.String() != `{"error":"Unauthorized"}` {
			t.Errorf("%s: %d %s, attesi 401 e Unauthorized", nome, rec.Code, rec.Body)
		}
	}
	// Il token di login con l'api-token del pannello non basta.
	if rec := conToken(g, "GET", rotta, tok["login"]); rec.Code != 401 {
		t.Errorf("api-token di login: %d %s, atteso 401", rec.Code, rec.Body)
	}
	for _, metodo := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		if rec := conBearer(g, metodo, rotta, "Bearer "+tok["n8n"]); rec.Code < 400 {
			t.Errorf("%s %s: %d, atteso un errore", metodo, rotta, rec.Code)
		}
	}
	n8n := tok["n8n"]
	if rec := conToken(g, "GET", "/api/admin/peer/list?page=1&page_size=10", n8n); rec.Code == 200 && strings.Contains(rec.Body.String(), `"code":0`) {
		t.Errorf("il token di integrazione apre il pannello: %s", rec.Body)
	}
	if rec := conBearer(g, "GET", "/api/admin/peer/list?page=1&page_size=10", "Bearer "+n8n); strings.Contains(rec.Body.String(), `"code":0`) {
		t.Errorf("il token di integrazione apre il pannello: %s", rec.Body)
	}
	for _, r := range []struct{ metodo, rotta string }{{"POST", "/api/currentUser"}, {"GET", "/api/user/info"}, {"GET", "/api/audit/conn/active?id=999000111&session_id=1&conn_type=0"}, {"POST", "/api/ab"}} {
		if rec := conBearer(g, r.metodo, r.rotta, "Bearer "+n8n); rec.Code != 401 {
			t.Errorf("%s %s col token di integrazione: %d %s, atteso 401", r.metodo, r.rotta, rec.Code, rec.Body)
		}
	}
	for nome, k := range tok {
		if strings.Contains(registro.String(), k) {
			t.Errorf("nel log c'e' il token %s:\n%s", nome, registro)
		}
	}
	if !strings.Contains(registro.String(), "token di integrazione revocato scaduto o revocato") {
		t.Errorf("manca il warn col nome del token revocato:\n%s", registro)
	}
}
