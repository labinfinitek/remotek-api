package contratto

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// anonymous risponde ai quattro passi anonimi come l'istanza di riferimento,
// con due valori scelti dal test: data di /api/version (JSON grezzo; il golden
// vale "2.7\n" e si confronta per forma) ed error del 401 di /api/ab.
func anonymous(version, abError string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			reply(w, http.StatusOK, ctJSON, `{"code":0,"data":`+version+`,"message":"success"}`)
		case "/api/login-options":
			reply(w, http.StatusOK, ctJSON, `["common-oidc/[{\"name\":\"webauth\"}]","oidc/webauth"]`)
		case "/api/ab":
			reply(w, http.StatusUnauthorized, ctJSON, `{"error":"`+abError+`"}`)
		default:
			reply(w, http.StatusNotFound, ctText, "404 not found")
		}
	}
}

// reply scrive una risposta con stato, Content-Type e corpo dati.
func reply(w http.ResponseWriter, status int, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// TestRunStepReportsDifference e' la prova permanente del rosso: RunStep da'
// una differenza se la risposta cambia, e solo allora; "forma" guarda i tipi
// del corpo, non i valori.
func TestRunStepReportsDifference(t *testing.T) {
	cases := []struct{ step, version, abError, want string }{
		{"version", `"remotek-prova"`, "Unauthorized", ""},
		{"version", "null", "Unauthorized", `"data": "null"`},
		{"login-options", `"2.7\n"`, "Unauthorized", ""},
		{"non-autenticato", `"2.7\n"`, "Unauthorized", ""},
		{"non-autenticato", `"2.7\n"`, "Altro", `"error": "Altro"`},
		{"audit-conn-active-404", `"2.7\n"`, "Unauthorized", ""},
	}
	fsys := os.DirFS("testdata/client-1.4.9")
	for _, c := range cases {
		s, err := LoadStep(fsys, c.step)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(anonymous(c.version, c.abError))
		d, err := RunStep(t.Context(), NewClient(), srv.URL, s, nil)
		srv.Close()
		if err != nil || (c.want == "" && d != "") || !strings.Contains(d, c.want) {
			t.Errorf("RunStep(%s) con %s e %s = %q, %v; attesa differenza con %q", c.step, c.version, c.abError, d, err, c.want)
		}
	}
}

// TestRunStepErrors prova ogni uscita con errore di RunStep e il limite di 4
// MiB preso esatto, che passa: la risposta e' letta per intero.
func TestRunStepErrors(t *testing.T) {
	full := bytes.Repeat([]byte("a"), maxResponseBytes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/interrotta":
			panic(http.ErrAbortHandler)
		case "/troncata":
			w.Header().Set("Content-Length", "10")
			_, _ = io.WriteString(w, "ab")
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		case "/compressa":
			w.Header().Set("Content-Encoding", "gzip")
		case "/limite":
			reply(w, http.StatusOK, ctText, string(full))
		case "/oltre":
			reply(w, http.StatusOK, ctText, string(full)+"a")
		}
	}))
	defer srv.Close()
	step := func(path, compare, golden string) *Step {
		return &Step{Name: "passo", Request: Request{Method: http.MethodGet, Path: path}, Meta: Meta{Compare: compare}, Golden: []byte(golden)}
	}
	cases := []struct {
		s    *Step
		want string
	}{
		{step("/__TOKEN__", compareExact, ""), "percorso: manca 'token'"},
		{step("/interrotta", compareExact, ""), "richiesta: Get "},
		{step("/troncata", compareExact, ""), "lettura della risposta: unexpected EOF"},
		{step("/compressa", compareExact, ""), "risposta compressa non prevista (Content-Encoding gzip)"},
		{step("/oltre", compareExact, ""), "risposta oltre 4194304 byte"},
		{step("/", "Esatto", ""), `confronto "Esatto" sconosciuto`},
		{step("/", compareShape, "x"), "confronto forma: decodifica"},
		{step("/", compareShape, "[]"), "confronto forma: la busta non e' un oggetto"},
	}
	c := NewClient()
	for _, tc := range cases {
		if d, err := RunStep(t.Context(), c, srv.URL, tc.s, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("RunStep(%s) = %d byte di differenza, %v; atteso errore con %q", tc.s.Request.Path, len(d), err, tc.want)
		}
	}
	golden := mustSerialize(t, Envelope(http.StatusOK, ctText, full, nil, ""))
	if d, err := RunStep(t.Context(), c, srv.URL, step("/limite", compareExact, string(golden)), nil); d != "" || err != nil {
		t.Errorf("RunStep di 4 MiB = %d byte di differenza, %v", len(d), err)
	}
}

// TestRunStepUsesVars: RunStep salva in vars il valore che Extract indica,
// preso dal corpo grezzo, e normalizza col nome utente che vars aveva prima
// dell'estrazione, come il giro del registratore.
func TestRunStepUsesVars(t *testing.T) {
	srv := httptest.NewServer(anonymous(`"collaudo"`, "Unauthorized"))
	defer srv.Close()
	s, err := LoadStep(os.DirFS("testdata/client-1.4.9"), "version")
	if err != nil {
		t.Fatal(err)
	}
	s.Meta.Extract = map[string]string{"versione": "data"}
	vars, c := map[string]string{"utente": "collaudo"}, NewClient()
	if d, err := RunStep(t.Context(), c, srv.URL, s, vars); d != "" || err != nil || vars["versione"] != "collaudo" {
		t.Errorf("RunStep(version) = %q, %v; contesto %v", d, err, vars)
	}
	// al byte il golden ha data "2.7\n", la risposta l'utente normalizzato
	s.Meta.Compare = compareExact
	if d, err := RunStep(t.Context(), c, srv.URL, s, vars); err != nil || !strings.Contains(d, `"data": "__UTENTE__"`) {
		t.Errorf("RunStep(version) al byte = %q, %v; attesa differenza con __UTENTE__", d, err)
	}
	// la busta usa l'utente di prima: "collaudo" arriva in vars solo dopo
	s.Meta.Extract, vars = map[string]string{"utente": "data"}, map[string]string{"utente": "altro"}
	if d, err := RunStep(t.Context(), c, srv.URL, s, vars); err != nil || strings.Contains(d, "__UTENTE__") || vars["utente"] != "collaudo" {
		t.Errorf("RunStep(version) con estrai utente = %q, %v; contesto %v", d, err, vars)
	}
}

// TestExtract: si estrae solo una stringa non vuota da un corpo che e' un
// solo oggetto JSON; altrimenti la variabile resta com'era.
func TestExtract(t *testing.T) {
	keys := map[string]string{"token": "access_token", "guid": "guid"}
	for _, c := range [][2]string{
		{`{"access_token":"abc","guid":"1-2-0"}`, "abc 1-2-0"},
		{`{"access_token":"","guid":"1-2-0"}`, "prima 1-2-0"},
		{`{"access_token":5,"guid":null}`, "prima prima"},
		{`[{"access_token":"abc"}]`, "prima prima"},
		{`{"access_token":"abc"} {}`, "prima prima"},
	} {
		vars := map[string]string{"token": "prima", "guid": "prima"}
		extract([]byte(c[0]), keys, vars)
		if got := vars["token"] + " " + vars["guid"]; got != c[1] {
			t.Errorf("extract(%s) = %q, atteso %q", c[0], got, c[1])
		}
	}
	extract([]byte(`{"access_token":"abc"}`), nil, nil) // senza chiavi vars non si tocca: nil non e' un errore
}

func TestNewClient(t *testing.T) {
	c := NewClient()
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || !tr.DisableCompression || c.Timeout != 12*time.Second || !errors.Is(c.CheckRedirect(nil, nil), http.ErrUseLastResponse) {
		t.Errorf("NewClient() = %+v", c)
	}
}
