package contratto

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// fakeT fa le veci di *testing.T nei test dei fallimenti di Run: scrive in
// log il nome di ogni sottotest avviato e i messaggi di Errorf e Fatalf col
// nome del test, senza far fallire il test vero. Fatalf non ferma nulla: dopo
// un Fatalf Run torna da sola.
type fakeT struct {
	ctx  context.Context
	name string
	log  *[]string
}

func (f fakeT) Helper() {}

func (f fakeT) Context() context.Context { return f.ctx }

func (f fakeT) Errorf(format string, args ...any) {
	*f.log = append(*f.log, f.name+": "+fmt.Sprintf(format, args...))
}

func (f fakeT) Fatalf(format string, args ...any) { f.Errorf(format, args...) }

func (f fakeT) Run(name string, fn func(fakeT)) bool {
	sub := fakeT{f.ctx, path.Join(f.name, name), f.log}
	*f.log = append(*f.log, sub.name)
	fn(sub)
	return true
}

// received e' cio' che il server finto di extractStub ha ricevuto.
type received struct {
	header http.Header
	query  string
	body   []byte
}

// extractStub risponde al login di twoSteps con un token e al passo dopo con
// 204; cio' che ha ricevuto quest'ultimo va su seen se c'e' posto.
func extractStub(seen chan<- received) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login" {
			reply(w, http.StatusOK, ctJSON, `{"access_token":"abc","type":"access_token"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		select {
		case seen <- received{r.Header.Clone(), r.URL.RawQuery, body}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// twoSteps e' una MapFS con scenario.json e due passi del gruppo utente,
// scritti con Serialize come dal registratore: login estrae il token da
// access_token e dopo lo manda in Authorization, in un POST senza corpo e
// con la query fuori dall'ordine alfabetico.
func twoSteps(t *testing.T) fstest.MapFS {
	t.Helper()
	steps := []any{map[string]any{"nome": "login", "gruppo": "utente"}, map[string]any{"nome": "dopo", "gruppo": "utente"}}
	files := map[string]any{
		"scenario.json":              map[string]any{"client": "prova", "passi": steps},
		"login/richiesta.json":       map[string]any{"metodo": "POST", "percorso": "/api/login", "query": []any{}, "intestazioni": map[string]any{}, "corpo": nil, "corpo_vuoto": true},
		"login/meta.json":            map[string]any{"confronto": "esatto", "estrai": map[string]any{"token": "access_token"}},
		"login/risposta.golden.json": map[string]any{"stato": 200, "content_type": ctJSON, "corpo_tipo": "json", "corpo": map[string]any{"access_token": PlaceholderToken, "type": "access_token"}},
		"dopo/richiesta.json":        map[string]any{"metodo": "POST", "percorso": "/api/dopo", "query": []any{[]any{"z", "2"}, []any{"a", "1"}}, "intestazioni": map[string]any{"Authorization": "Bearer __TOKEN__"}, "corpo": nil, "corpo_vuoto": true},
		"dopo/meta.json":             map[string]any{"confronto": "esatto"},
		"dopo/risposta.golden.json":  map[string]any{"stato": 204, "content_type": "", "corpo_tipo": "vuoto", "corpo": nil},
	}
	fsys := fstest.MapFS{}
	for name, v := range files {
		fsys[name] = &fstest.MapFile{Data: mustSerialize(t, v)}
	}
	return fsys
}

// TestRunAgainstFaithfulStub riesegue con *testing.T il gruppo anonime contro
// un server finto che risponde come l'istanza di riferimento, version con un
// altro valore che "forma" accetta.
func TestRunAgainstFaithfulStub(t *testing.T) {
	srv := httptest.NewServer(anonymous(`"remotek-prova"`, "Unauthorized"))
	defer srv.Close()
	Run(t, Options{BaseURL: srv.URL, Golden: os.DirFS("testdata/client-1.4.9"), Groups: []string{"anonime"}})
}

// TestRunFailures prova con fakeT come fallisce Run: scenario illeggibile,
// gruppo sconosciuto o nessun gruppo fermano tutto; golden mancante o
// segnaposto senza valore fermano il passo; una differenza fa fallire il
// passo e gli altri proseguono, nell'ordine dello scenario.
func TestRunFailures(t *testing.T) {
	anon := httptest.NewServer(anonymous(`"2.7\n"`, "Altro"))
	defer anon.Close()
	two := httptest.NewServer(extractStub(nil))
	defer two.Close()
	goldens := os.DirFS("testdata/client-1.4.9")
	noGolden, noExtract := twoSteps(t), twoSteps(t)
	delete(noGolden, "dopo/risposta.golden.json")
	noExtract["login/meta.json"] = &fstest.MapFile{Data: []byte(`{"confronto":"esatto"}`)}
	cases := []struct {
		name string
		o    Options
		want []string // prefissi, uno per riga di log
	}{
		{"scenario assente", Options{BaseURL: anon.URL, Golden: fstest.MapFS{}, Groups: []string{"anonime"}}, []string{"run: scenario: lettura: "}},
		{"gruppo sconosciuto", Options{BaseURL: anon.URL, Golden: goldens, Groups: []string{"anonime", "nessuno"}}, []string{`run: scenario: gruppo "nessuno" assente dallo scenario`}},
		{"nessun gruppo", Options{BaseURL: anon.URL, Golden: goldens}, []string{"run: scenario: nessun passo da rieseguire"}},
		{"golden mancante", Options{BaseURL: two.URL, Golden: noGolden, Groups: []string{"utente"}}, []string{"run/login", "run/dopo", "run/dopo: golden mancante o non valido in un gruppo attivo: lettura: "}},
		{"segnaposto senza valore", Options{BaseURL: two.URL, Golden: noExtract, Groups: []string{"utente"}}, []string{"run/login", "run/dopo", "run/dopo: intestazione Authorization: manca 'token'"}},
		{"differenza", Options{BaseURL: anon.URL, Golden: goldens, Groups: []string{"anonime"}}, []string{"run/version", "run/login-options", "run/non-autenticato", "run/non-autenticato: prima differenza alla riga 4", "run/audit-conn-active-404"}},
	}
	for _, c := range cases {
		var log []string
		Run(fakeT{t.Context(), "run", &log}, c.o)
		ok := len(log) == len(c.want)
		for i := 0; ok && i < len(log); i++ {
			ok = strings.HasPrefix(log[i], c.want[i])
		}
		if !ok {
			t.Errorf("%s: Run ha scritto\n%s\natteso (prefissi)\n%s", c.name, strings.Join(log, "\n"), strings.Join(c.want, "\n"))
		}
	}
}

// TestRunExtractsAndSubstitutes: Run porta il token estratto dal login al
// passo dopo, che lo manda in Authorization con la query nell'ordine del
// file, senza Accept-Encoding e, POST senza corpo, con Content-Length: 0 come
// il registratore. Run lavora su una copia di o.Vars, anche se e' nil.
func TestRunExtractsAndSubstitutes(t *testing.T) {
	seen := make(chan received, 1)
	srv := httptest.NewServer(extractStub(seen))
	defer srv.Close()
	vars := map[string]string{"utente": "collaudo"}
	Run(t, Options{BaseURL: srv.URL, Golden: twoSteps(t), Groups: []string{"utente"}, Vars: vars})
	Run(t, Options{BaseURL: srv.URL, Golden: twoSteps(t), Groups: []string{"utente"}})
	if t.Failed() {
		return
	}
	got := <-seen
	h := got.header
	if len(vars) != 1 || h.Get("Authorization") != "Bearer abc" || got.query != "z=2&a=1" || len(got.body) != 0 || h.Get("Content-Length") != "0" || h.Get("Accept-Encoding") != "" {
		t.Errorf("contesto %v, ricevuto %+v", vars, got)
	}
}

// TestSelectSteps: i passi restano nell'ordine dello scenario, non in quello
// dei gruppi. Gli errori di selectSteps sono in TestRunFailures.
func TestSelectSteps(t *testing.T) {
	sc := Scenario{Steps: []ScenarioStep{{"a1", "a"}, {"b1", "b"}, {"a2", "a"}}}
	for groups, want := range map[string][]ScenarioStep{"b a": sc.Steps, "a": {{"a1", "a"}, {"a2", "a"}}} {
		if got, err := selectSteps(sc, strings.Fields(groups)); err != nil || !slices.Equal(got, want) {
			t.Errorf("selectSteps(%s) = %v, %v; atteso %v", groups, got, err, want)
		}
	}
}
