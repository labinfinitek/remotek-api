package main

// Test del contratto sul router vero dell'API. Sta in package main solo
// perche' InitGlobal e' qui: e' provvisorio, finche' il bootstrap non esce
// da cmd/ (test/contratto/README.md, sezione Stato).
//
// In questo pacchetto UN SOLO test di primo livello puo' chiamare
// InitGlobal: lo stato globale non ha guardie, e una seconda chiamata
// aprirebbe un altro file di log, avvierebbe un'altra goroutine del limiter
// e sovrascriverebbe i servizi. Niente t.Parallel: t.Chdir e t.Setenv lo
// vietano.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/global"
	apihttp "github.com/lejianwen/rustdesk-api/v2/http"
	"github.com/lejianwen/rustdesk-api/v2/test/contratto"
)

// Utente di collaudo e dispositivo con cui il client fa login, come nella
// tabella del registratore (voce login): nome minuscolo e senza spazi, che
// la registrazione non cambia.
const (
	seedUsername = "collaudo-contratto"
	peerID       = "999000111"
	peerUUID     = "cmVtb3Rlay1jb2xsYXVkby11dWlk"
)

// maxSeedResponseBytes limita la lettura delle risposte del seme, come fa la
// libreria del contratto con le risposte dei passi.
const maxSeedResponseBytes = 4 << 20

// TestContract riesegue i golden dei gruppi attivi di test/contratto sul
// router vero, servito da httptest dopo il bootstrap di produzione
// (InitGlobal) in una cartella temporanea.
func TestContract(t *testing.T) {
	// go test parte da cmd/: la radice del repo si prende prima di spostarsi.
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("radice del repo: %v", err)
	}

	// Il codice apre percorsi relativi alla cartella corrente
	// (./data/rustdeskapi.db, ./runtime/log.txt, resources/templates/*,
	// resources/version, resources/i18n, ./conf/admin/hello.html): nella
	// sandbox si risolvono tutti li' dentro e nel repo non si scrive nulla.
	// La cartella corrente resta la sandbox fino alla fine del test, perche'
	// sqlite apre le connessioni quando servono, col percorso relativo.
	sandbox := t.TempDir()
	for _, dir := range []string{"data", "runtime"} {
		if err := os.MkdirAll(filepath.Join(sandbox, dir), 0o750); err != nil {
			t.Fatalf("sandbox: %v", err)
		}
	}
	for _, dir := range []string{"resources", "conf"} {
		if err := os.Symlink(filepath.Join(root, dir), filepath.Join(sandbox, dir)); err != nil {
			t.Fatalf("sandbox: %v", err)
		}
	}
	t.Chdir(sandbox)

	// Configurazione dell'istanza di riferimento da cui vengono i golden,
	// fissata apposta: se cambiano i default nel codice, i golden non si
	// spostano. Per questo il test NON prova i default sicuri, che avranno
	// test propri. Il modo test di gin non stampa nulla e non cambia le
	// risposte. Log a warn: la password casuale di admin creata dalla
	// migrazione va nel log a livello info e non deve finire nel log
	// pubblico della CI.
	t.Setenv("RUSTDESK_API_LANG", "en")
	t.Setenv("RUSTDESK_API_APP_WEB_CLIENT", "0")
	t.Setenv("RUSTDESK_API_APP_SHOW_SWAGGER", "0")
	t.Setenv("RUSTDESK_API_APP_WEB_SSO", "true")
	t.Setenv("RUSTDESK_API_APP_DISABLE_PWD_LOGIN", "false")
	t.Setenv("RUSTDESK_API_APP_CAPTCHA_THRESHOLD", "3")
	t.Setenv("RUSTDESK_API_APP_BAN_THRESHOLD", "10")
	t.Setenv("RUSTDESK_API_RUSTDESK_PERSONAL", "1")
	t.Setenv("RUSTDESK_API_GIN_MODE", "test")
	t.Setenv("RUSTDESK_API_LOGGER_LEVEL", "warn")
	// L'autoregistrazione invece e' accesa SOLO in questo processo, per
	// creare l'utente di collaudo via HTTP; l'istanza di riferimento e la
	// produzione la tengono spenta (app.register in conf/config.yaml).
	// Nessun endpoint del client la legge: app.register conta solo per
	// /api/admin/user/register e per le opzioni di login del pannello,
	// app.register-status solo per il primo.
	t.Setenv("RUSTDESK_API_APP_REGISTER", "true")
	t.Setenv("RUSTDESK_API_APP_REGISTER_STATUS", "1")

	global.ConfigPath = filepath.Join(root, "conf", "config.yaml")
	InitGlobal()

	srv := httptest.NewServer(apihttp.NewEngine())
	t.Cleanup(srv.Close)

	// I sottotest girano in sequenza nell'ordine del codice (-shuffle
	// mescola solo i test di primo livello): il seme precede gli scenari.
	// Con -run il filtro deve includere il seme, per esempio
	// -run 'TestContract/(seme|login)$': un sottotest escluso dal filtro non
	// gira e t.Run restituisce true, quindi i passi con utente fallirebbero
	// per un segnaposto senza valore.
	var user, password string
	if !t.Run("seme", func(t *testing.T) {
		c := contratto.NewClient()
		defer c.CloseIdleConnections()
		user, password = seedUser(t, c, srv.URL)
		checkLogin(t, c, srv.URL, user, password)
	}) {
		t.Fatalf("seme dell'utente di collaudo non riuscito: senza, gli scenari non hanno senso")
	}

	contratto.Run(t, contratto.Options{
		BaseURL: srv.URL,
		Golden:  os.DirFS(filepath.Join(root, "test", "contratto", "testdata", "client-1.4.9")),
		Groups:  []string{"anonime", "non-implementate", "utente", "peer", "login-errato"},
		Vars:    map[string]string{"utente": user, "password": password},
	})
}

// seedUser crea l'utente di collaudo con POST /api/admin/user/register: non
// admin, gruppo 1, email vuota come quello dell'istanza di riferimento. Il
// seme passa da HTTP e non da service.* perche' deve sopravvivere al
// refactor dei servizi (ADR-0014). La password e' casuale a ogni esecuzione,
// mai una costante (repo pubblico, gitleaks), e non si stampa mai; in errore
// si stampano solo code e message, perche' il corpo contiene un token.
func seedUser(t *testing.T, c *http.Client, baseURL string) (user, password string) {
	t.Helper()
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("password casuale: %v", err)
	}
	user, password = seedUsername, hex.EncodeToString(secret) // 32 caratteri: il form accetta 4-32
	status, body := postJSON(t, c, baseURL+"/api/admin/user/register", "", map[string]any{
		"username": user, "email": "", "password": password, "confirm_password": password,
	})
	if code, _ := body["code"].(json.Number); status != http.StatusOK || code != "0" {
		t.Fatalf("registrazione: stato %d, code %v, message %v", status, body["code"], body["message"])
	}
	return user, password
}

// checkLogin rifa il giro di --verifica del registratore col corpo che manda
// il client: login, currentUser col token, logout, e currentUser di nuovo,
// che deve dare 401 perche' il logout cancella il token. Non stampa ne' il
// token ne' la password.
func checkLogin(t *testing.T, c *http.Client, baseURL, user, password string) {
	t.Helper()
	status, body := postJSON(t, c, baseURL+"/api/login", "", map[string]any{
		"username":   user,
		"password":   password,
		"id":         peerID,
		"uuid":       peerUUID,
		"autoLogin":  true,
		"type":       "account",
		"deviceInfo": map[string]string{"os": "windows", "type": "client", "name": "REMOTEK-COLLAUDO"},
	})
	token, _ := body["access_token"].(string)
	u, _ := body["user"].(map[string]any)
	_, info := u["info"].(map[string]any)
	if status != http.StatusOK || body["type"] != "access_token" || token == "" ||
		u["name"] != user || u["email"] != "" || u["is_admin"] != false || !info {
		t.Fatalf("login: stato %d, type %v, token presente %t, name %v, email %v, is_admin %v, info oggetto %t, error %v",
			status, body["type"], token != "", u["name"], u["email"], u["is_admin"], info, body["error"])
	}
	device := map[string]any{"id": peerID, "uuid": peerUUID}
	for _, step := range []struct {
		path string
		want int
	}{
		{"/api/currentUser", http.StatusOK},
		{"/api/logout", http.StatusOK},
		{"/api/currentUser", http.StatusUnauthorized},
	} {
		if got, _ := postJSON(t, c, baseURL+step.path, token, device); got != step.want {
			t.Fatalf("%s: stato %d, atteso %d", step.path, got, step.want)
		}
	}
}

// postJSON manda a target un POST con corpo JSON e, se token non e' vuoto,
// Authorization: Bearer; restituisce lo stato e il corpo se e' un oggetto
// JSON (altrimenti nil, che si legge come mappa vuota).
func postJSON(t *testing.T, c *http.Client, target, token string, payload any) (int, map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("corpo: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("richiesta: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSeedResponseBytes+1))
	if err != nil {
		t.Fatalf("risposta di %s: %v", target, err)
	}
	if len(raw) > maxSeedResponseBytes {
		t.Fatalf("risposta di %s oltre %d byte", target, maxSeedResponseBytes)
	}
	v, _ := contratto.Decode(raw) // corpo non JSON: v e' nil
	obj, _ := v.(map[string]any)
	return resp.StatusCode, obj
}
