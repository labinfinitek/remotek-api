package contratto

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// recorded sono i passi del gruppo anonime, i primi di scenario.json. Gli
// altri 41 passi registrati li carica TestContract in cmd/.
var recorded = []string{"version", "login-options", "non-autenticato", "audit-conn-active-404"}

func TestLoadScenario(t *testing.T) {
	sc, err := LoadScenario(os.DirFS("testdata/client-1.4.9"))
	if err != nil || sc.Client != "1.4.9" || len(sc.Steps) < len(recorded) {
		t.Fatalf("LoadScenario = %+v, %v", sc, err)
	}
	for i, name := range recorded {
		if sc.Steps[i] != (ScenarioStep{Name: name, Group: "anonime"}) {
			t.Errorf("passo %d = %+v, atteso %s del gruppo anonime", i, sc.Steps[i], name)
		}
	}
	if _, err := LoadScenario(fstest.MapFS{}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("LoadScenario senza scenario.json: errore %v, atteso fs.ErrNotExist", err)
	}
}

// TestLoadStepRecorded legge i passi di recorded: golden uguale al file,
// nessuna chiave annidata, confronto "forma" solo per version.
func TestLoadStepRecorded(t *testing.T) {
	fsys := os.DirFS("testdata/client-1.4.9")
	for _, name := range recorded {
		s, err := LoadStep(fsys, name)
		if err != nil {
			t.Fatalf("LoadStep(%s): %v", name, err)
		}
		golden, err := fs.ReadFile(fsys, path.Join(name, "risposta.golden.json"))
		compare := compareExact
		if name == "version" {
			compare = compareShape
		}
		if err != nil || s.Name != name || s.Meta.Compare != compare || !bytes.Equal(s.Golden, golden) || len(s.NestedKeys) != 0 {
			t.Errorf("LoadStep(%s) = %+v (lettura del golden: %v)", name, s, err)
		}
	}
	if _, err := LoadStep(fsys, "non-esiste"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("LoadStep(non-esiste): errore %v, atteso fs.ErrNotExist", err)
	}
}

// stepFiles sono i tre file di un passo valido, con chiavi annidate fuori ordine.
var stepFiles = map[string]string{
	"richiesta.json":       `{"corpo":{"u":"__UTENTE__"},"corpo_vuoto":true,"intestazioni":{"A":"b"},"metodo":"POST","percorso":"/api/ab","query":[["q","1"]]}`,
	"meta.json":            `{"confronto":"esatto","estrai":{"token":"access_token"},"nota":""}`,
	"risposta.golden.json": `{"corpo_annidato":{"e":1,"d":{},"c":null,"b":"x","a":[]},"corpo_tipo":"json"}`,
}

// stepFS mette stepFiles sotto passo/ in una MapFS, con il file file
// sostituito da data; data vuoto toglie il file.
func stepFS(file, data string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for f, d := range stepFiles {
		if f != file {
			fsys["passo/"+f] = &fstest.MapFile{Data: []byte(d)}
		} else if data != "" {
			fsys["passo/"+f] = &fstest.MapFile{Data: []byte(data)}
		}
	}
	return fsys
}

func TestLoadStepSynthetic(t *testing.T) {
	s, err := LoadStep(stepFS("", ""), "passo")
	want := &Step{
		Name:       "passo",
		Request:    Request{Method: "POST", Path: "/api/ab", Query: [][]string{{"q", "1"}}, Headers: map[string]string{"A": "b"}, Body: json.RawMessage(`{"u":"__UTENTE__"}`), EmptyBody: true},
		Meta:       Meta{Compare: compareExact, Extract: map[string]string{"token": "access_token"}},
		Golden:     []byte(stepFiles["risposta.golden.json"]),
		NestedKeys: []string{"a", "b", "c", "d", "e"},
	}
	if err != nil || !reflect.DeepEqual(s, want) {
		t.Errorf("LoadStep = %+v, %v; atteso %+v", s, err, want)
	}
}

// TestLoadStepRejects rompe un file alla volta: ogni controllo di LoadStep
// deve dare il suo errore, e solo un file assente da' fs.ErrNotExist.
func TestLoadStepRejects(t *testing.T) {
	cases := []struct{ name, file, data, want string }{
		{"richiesta assente", "richiesta.json", "", "passo/richiesta.json"},
		{"meta assente", "meta.json", "", "passo/meta.json"},
		{"golden assente", "risposta.golden.json", "", "passo/risposta.golden.json"},
		{"richiesta non JSON", "richiesta.json", `{"metodo":`, "passo/richiesta.json"},
		{"metodo vuoto", "richiesta.json", `{"percorso":"/api/ab"}`, "metodo vuoto"},
		{"percorso relativo", "richiesta.json", `{"metodo":"GET","percorso":"api/ab"}`, "senza / iniziale"},
		{"coppia di query lunga 3", "richiesta.json", `{"metodo":"GET","percorso":"/x","query":[["a","1","2"]]}`, "3 elementi"},
		{"coppia di query lunga 1", "richiesta.json", `{"metodo":"GET","percorso":"/x","query":[["a"]]}`, "1 elementi"},
		{"meta non JSON", "meta.json", `[`, "passo/meta.json"},
		{"confronto sconosciuto", "meta.json", `{"confronto":"Esatto"}`, `confronto "Esatto" sconosciuto`},
		{"confronto assente", "meta.json", `{}`, `confronto "" sconosciuto`},
		{"golden non JSON", "risposta.golden.json", "404 not found", "passo/risposta.golden.json"},
		{"golden non oggetto", "risposta.golden.json", `[]`, "la busta non e' un oggetto"},
		{"corpo_annidato non oggetto", "risposta.golden.json", `{"corpo_annidato":"x","corpo_tipo":"json"}`, "corpo_annidato non e' un oggetto"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadStep(stepFS(c.file, c.data), "passo")
			if err == nil || !strings.Contains(err.Error(), c.want) || errors.Is(err, fs.ErrNotExist) != (c.data == "") {
				t.Errorf("LoadStep: errore %v, atteso con %q", err, c.want)
			}
		})
	}
}
