package contratto

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// testVars e' il contesto dello scenario dei test di questo file. Gli attesi
// sono l'uscita del registratore con lo stesso contesto: sostituisci(),
// corpo_in_byte() e le righe di manda() che costruiscono l'URL.
var testVars = map[string]string{"token": "abc", "guid": "1-2-0", "utente": "collaudo", "password": "segreta"}

func TestSubstitute(t *testing.T) {
	for _, c := range [][2]string{
		{"__TOKEN__ __GUID__ __UTENTE__ __PASSWORD__", "abc 1-2-0 collaudo segreta"},
		{"__GUID__/__GUID__", "1-2-0/1-2-0"},
		{"__UTENTE_ nessuno", "__UTENTE_ nessuno"},
	} {
		if got, err := substitute(c[0], testVars); got != c[1] || err != nil {
			t.Errorf("substitute(%q) = %q, %v; atteso %q", c[0], got, err, c[1])
		}
	}
	for key := range testVars {
		vars := maps.Clone(testVars)
		vars[key] = ""
		_, err := substitute("x __TOKEN__ __GUID__ __UTENTE__ __PASSWORD__", vars)
		if want := "manca '" + key + "' nel contesto dello scenario"; err == nil || err.Error() != want {
			t.Errorf("substitute senza %s: errore %v, atteso %q", key, err, want)
		}
	}
}

// TestBuild prova URL, intestazioni e corpo di Build; la base finisce con due
// / che cadono come con rstrip("/").
func TestBuild(t *testing.T) {
	const base = "http://contratto.example//"
	auth := map[string]string{"Authorization": "Bearer __TOKEN__", "Content-Type": "application/json"}
	cases := []struct {
		name string
		r    Request
		url  string
		body any // nil: nessun corpo; http.NoBody; string: i byte del corpo
	}{
		{"query nell'ordine del file", Request{Method: "GET", Path: "/api/ab/peers", Query: [][]string{{"current", "1"}, {"pageSize", "100"}, {"ab", "__GUID__"}}, Headers: auth},
			"http://contratto.example/api/ab/peers?current=1&pageSize=100&ab=1-2-0", nil},
		{"escape come quote e urlencode", Request{Method: "GET", Path: "/api/ab/peer/__GUID__/a b:c@d+$&,;=é%?#", Query: [][]string{{"__GUID__", "__GUID__"}, {"a b", "x&y=z+é w/~"}}},
			"http://contratto.example/api/ab/peer/1-2-0/a%20b%3Ac%40d%2B%24%26%2C%3B%3D%C3%A9%25%3F%23?__GUID__=1-2-0&a+b=x%26y%3Dz%2B%C3%A9+w%2F~", nil},
		{"corpo JSON con segnaposto", Request{Method: "POST", Path: "/api/login", Headers: auth, EmptyBody: true, Body: json.RawMessage(`{"__UTENTE__":["__PASSWORD__",true,null,"<&>","è"],"session_id":9007199254740993,"username":"__UTENTE__"}`)},
			"http://contratto.example/api/login", `{"__UTENTE__":["segreta",true,null,"<&>","è"],"session_id":9007199254740993,"username":"collaudo"}`},
		{"corpo lista", Request{Method: "DELETE", Path: "/x", Body: json.RawMessage(` [1, "__GUID__"] `)}, "http://contratto.example/x", `[1,"1-2-0"]`},
		{"corpo vuoto", Request{Method: "POST", Path: "/x", EmptyBody: true}, "http://contratto.example/x", http.NoBody},
		{"corpo null e vuoto", Request{Method: "POST", Path: "/x", Body: json.RawMessage(" null\n"), EmptyBody: true}, "http://contratto.example/x", http.NoBody},
		{"GET senza corpo", Request{Method: "GET", Path: "/x", Body: json.RawMessage("null")}, "http://contratto.example/x", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := c.r.Build(t.Context(), base, testVars)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if req.Method != c.r.Method || req.URL.String() != c.url {
				t.Errorf("richiesta %s %s, attesa %s %s", req.Method, req.URL, c.r.Method, c.url)
			}
			header := http.Header{}
			if c.r.Headers != nil {
				header = http.Header{"Authorization": {"Bearer abc"}, "Content-Type": {"application/json"}}
			}
			if !reflect.DeepEqual(req.Header, header) {
				t.Errorf("intestazioni %v, attese %v", req.Header, header)
			}
			checkBody(t, req, c.body)
		})
	}
}

// TestBuildRejects: un segnaposto senza valore e' un errore ovunque stia
// (vars nil), come una richiesta che Build non sa costruire. Con piu' valori
// mancanti l'errore e' sempre il primo: intestazioni prima del corpo, come in
// manda(), e chiavi in ordine.
func TestBuildRejects(t *testing.T) {
	cases := []struct {
		want string
		r    Request
	}{
		{"percorso: manca 'guid'", Request{Method: "GET", Path: "/api/ab/peer/__GUID__"}},
		{"query ab: manca 'token'", Request{Method: "GET", Path: "/x", Query: [][]string{{"ab", "__TOKEN__"}}}},
		{"intestazione Authorization: manca 'token'", Request{Method: "POST", Path: "/x", Headers: map[string]string{"Z": "__GUID__", "Authorization": "Bearer __TOKEN__"}, Body: json.RawMessage(`{"u":"__UTENTE__"}`)}},
		{"corpo: manca 'password'", Request{Method: "POST", Path: "/x", Body: json.RawMessage(`{"b":"__UTENTE__","a":[{"c":"__PASSWORD__"}]}`)}},
		{"corpo: decodifica", Request{Method: "POST", Path: "/x", Body: json.RawMessage(`{`)}},
		{"coppia della query", Request{Method: "GET", Path: "/x", Query: [][]string{{"ab"}}}},
		{"invalid method", Request{Method: "GE T", Path: "/x"}},
	}
	for _, c := range cases {
		if _, err := c.r.Build(t.Context(), "http://contratto.example", nil); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Build(%+v): errore %v, atteso con %q", c.r, err, c.want)
		}
	}
}

// checkBody confronta il corpo di req con want: nil (nessun corpo),
// http.NoBody o la stringa dei byte attesi, con Content-Length coerente.
func checkBody(t *testing.T, req *http.Request, want any) {
	t.Helper()
	s, isBytes := want.(string)
	if !isBytes {
		if req.Body != want || req.ContentLength != 0 {
			t.Errorf("corpo %#v di %d byte, atteso %#v", req.Body, req.ContentLength, want)
		}
		return
	}
	if req.Body == nil {
		t.Fatalf("nessun corpo, attesi %q", s)
	}
	got, err := io.ReadAll(req.Body)
	if err != nil || string(got) != s || req.ContentLength != int64(len(s)) {
		t.Errorf("corpo %q di %d byte (%v), atteso %q", got, req.ContentLength, err, s)
	}
}
