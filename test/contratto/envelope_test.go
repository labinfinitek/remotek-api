package contratto

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"testing"
)

const minRecordedSteps = 4 // minimo, i passi del gruppo anonime: trovarne meno vuol dire percorso sbagliato
const ctJSON, ctText = "application/json; charset=utf-8", "text/plain; charset=utf-8"

// TestEnvelopeReproducesGoldens prova Envelope e Serialize sui dati veri: da
// ogni golden ricostruisce i byte dell'API e pretende lo stesso file al byte.
func TestEnvelopeReproducesGoldens(t *testing.T) {
	fsys := os.DirFS("testdata/client-1.4.9")
	names, err := fs.Glob(fsys, "*/risposta.golden.json")
	if err != nil || len(names) < minRecordedSteps {
		t.Fatalf("trovati %d golden (errore %v), attesi almeno %d", len(names), err, minRecordedSteps)
	}
	for _, name := range names {
		want, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("lettura di %s: %v", name, err)
		}
		if d := Diff(want, reenvelope(t, want)); d != "" {
			t.Errorf("%s: %s", path.Dir(name), d)
		}
	}
}

// TestEnvelope prova ogni ramo di Envelope e OpenNested; gli attesi sono
// l'uscita di busta_risposta() del registratore sugli stessi byte.
func TestEnvelope(t *testing.T) {
	inner := mustMarshal(t, map[string]any{"peers": []any{map[string]any{"row_id": 12, "id": "999000111", "username": "collaudo"}}, "tags": []any{"ufficio"}, "tag_colors": `{"ufficio":4288585374}`})
	ab := string(mustMarshal(t, map[string]any{"data": string(inner), "total": 1})) // GET /api/ab: JSON in una stringa
	cases := []struct {
		name, ct, data, user string
		keys                 []string
		status               int
		want                 string
	}{
		{"json normalizzato", ctJSON, `{"id":7,"created_at":"2026-09-18 06:39:09","name":"collaudo","note":"2026-09-18 06:39:09"}`, "collaudo", nil, 200, `{"content_type":"application/json; charset=utf-8","corpo":{"created_at":"__TIMESTAMP__","id":"__ID__","name":"__UTENTE__","note":"2026-09-18 06:39:09"},"corpo_tipo":"json","stato":200}`},
		{"testo: date sostituite, utente no", ctText, "collaudo alle 2026-09-18 06:39:09 e 2026-09-18 06:40:00, non 2026-09-18T06:39:09", "collaudo", nil, 500, `{"content_type":"text/plain; charset=utf-8","corpo":"collaudo alle __TIMESTAMP__ e __TIMESTAMP__, non 2026-09-18T06:39:09","corpo_tipo":"testo","stato":500}`},
		{"testo non UTF-8", ctText, "a\xffb", "", nil, 400, `{"content_type":"text/plain; charset=utf-8","corpo":"a�b","corpo_tipo":"testo","stato":400}`},
		{"vuoto", "", "", "", nil, 204, `{"content_type":"","corpo":null,"corpo_tipo":"vuoto","stato":204}`},
		{"null", ctJSON, "null", "", nil, 200, `{"content_type":"application/json; charset=utf-8","corpo":null,"corpo_tipo":"json","stato":200}`},
		{"annidato", ctJSON, ab, "collaudo", []string{"data"}, 200, `{"content_type":"application/json; charset=utf-8","corpo":{"data":"__JSON:data__","total":1},"corpo_annidato":{"data":{"peers":[{"id":"999000111","row_id":"__ID__","username":"__UTENTE__"}],"tag_colors":{"__JSON__":{"ufficio":4288585374}},"tags":["ufficio"]}},"corpo_tipo":"json","stato":200}`},
		{"chiavi non apribili", ctJSON, `{"data":{"peers":[]},"text":"non json"}`, "", []string{"data", "text", "manca"}, 200, `{"content_type":"application/json; charset=utf-8","corpo":{"data":{"peers":[]},"text":"non json"},"corpo_tipo":"json","stato":200}`},
		{"tag_colors non JSON resta stringa", ctJSON, `{"ab":"{\"tag_colors\":\"rosso\",\"id\":3}"}`, "", []string{"ab"}, 200, `{"content_type":"application/json; charset=utf-8","corpo":{"ab":"__JSON:ab__"},"corpo_annidato":{"ab":{"id":"__ID__","tag_colors":"rosso"}},"corpo_tipo":"json","stato":200}`},
		{"annidati null e numero", ctJSON, `{"data":"null","n":"5"}`, "", []string{"data", "n"}, 200, `{"content_type":"application/json; charset=utf-8","corpo":{"data":"__JSON:data__","n":"__JSON:n__"},"corpo_annidato":{"data":null,"n":5},"corpo_tipo":"json","stato":200}`},
		{"corpo non oggetto", ctJSON, `["data"]`, "", []string{"data"}, 200, `{"content_type":"application/json; charset=utf-8","corpo":["data"],"corpo_tipo":"json","stato":200}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustSerialize(t, Envelope(c.status, c.ct, []byte(c.data), c.keys, c.user))
			if d := Diff(mustSerialize(t, mustDecode(t, c.want)), got); d != "" {
				t.Error(d)
			}
			if d := Diff(got, reenvelope(t, got)); d != "" { // prova reenvelope, che serve ai golden
				t.Errorf("reenvelope non inverte Envelope: %s", d)
			}
		})
	}
}

func TestOpenNestedLeavesInputIntact(t *testing.T) {
	body := mustDecode(t, `{"data":"{\"id\":3}"}`)
	before := mustSerialize(t, body)
	if _, nested := OpenNested(body, []string{"data"}, ""); len(nested) != 1 || !bytes.Equal(mustSerialize(t, body), before) {
		t.Errorf("OpenNested(%s): aperti %v, input dopo la chiamata %s", before, nested, mustSerialize(t, body))
	}
}

// reenvelope ricostruisce dalla busta serializzata b i byte dell'API e rifa' la
// busta: Normalize lascia com'e' un valore gia' normalizzato (tag_colors aperto incluso).
func reenvelope(t *testing.T, b []byte) []byte {
	t.Helper()
	env, _ := mustDecode(t, string(b)).(map[string]any)
	n, _ := env["stato"].(json.Number)
	status, err := n.Int64()
	if err != nil {
		t.Fatalf("stato %q: %v", n, err)
	}
	s, _ := env["corpo"].(string) // testo; con "vuoto" il corpo e' null: zero byte
	data := []byte(s)
	var keys []string
	if env["corpo_tipo"] == bodyJSON {
		nested, _ := env["corpo_annidato"].(map[string]any)
		if obj, ok := env["corpo"].(map[string]any); ok {
			for key, v := range nested {
				obj[key] = string(mustMarshal(t, v))
			}
		}
		keys = slices.Sorted(maps.Keys(nested))
		data = mustMarshal(t, env["corpo"])
	}
	ct, _ := env["content_type"].(string)
	return mustSerialize(t, Envelope(int(status), ct, data, keys, ""))
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}
