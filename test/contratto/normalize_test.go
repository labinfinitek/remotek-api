package contratto

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"testing"
)

// minGoldenFiles e' una soglia minima, non il numero dei file registrati: i 4
// passi del gruppo anonime (4 x 3 file) piu' scenario.json. Trovarne meno
// vuol dire percorso sbagliato, e un test che non legge nulla non deve
// passare in silenzio.
const minGoldenFiles = 13

// TestSerializeMatchesRecorder prova la parita' tra la forma canonica Go e
// quella del registratore sui dati veri: ogni file .json di testdata,
// decodificato e riscritto, deve tornare identico al byte.
func TestSerializeMatchesRecorder(t *testing.T) {
	fsys := os.DirFS("testdata")
	found := 0
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(name) != ".json" {
			return nil
		}
		found++
		want, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		v, err := Decode(want)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return nil
		}
		if got := mustSerialize(t, v); !bytes.Equal(got, want) {
			t.Errorf("%s: forma canonica diversa dal file del registratore\n--- file\n%s--- Go\n%s", name, want, got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("lettura di testdata: %v", err)
	}
	if found < minGoldenFiles {
		t.Fatalf("trovati %d file .json in testdata, attesi almeno %d", found, minGoldenFiles)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		user string
		want string
	}{
		{"token", `{"access_token":"abc"}`, "", `{"access_token":"__TOKEN__"}`},
		{"token vuoto resta", `{"access_token":""}`, "", `{"access_token":""}`},
		{"created_at valido", `{"created_at":"2026-09-18 06:39:09","updated_at":"2026-09-18T06:39:09Z"}`, "",
			`{"created_at":"__TIMESTAMP__","updated_at":"__TIMESTAMP__"}`},
		{"created_at non data resta", `{"created_at":"ieri"}`, "", `{"created_at":"ieri"}`},
		{"created_at con testo prima resta", `{"created_at":"x2026-09-18 06:39:09"}`, "", `{"created_at":"x2026-09-18 06:39:09"}`},
		{"id intero positivo", `{"id":5}`, "", `{"id":"__ID__"}`},
		{"id zero resta", `{"id":0}`, "", `{"id":0}`},
		{"id negativo resta", `{"id":-5}`, "", `{"id":-5}`},
		{"id meno zero resta", `{"id":-0}`, "", `{"id":-0}`},
		{"id con esponente resta", `{"id":1e3,"row_id":1E3}`, "", `{"id":1e3,"row_id":1E3}`},
		{"id stringa resta", `{"id":"999000111"}`, "", `{"id":"999000111"}`},
		{"id non intero resta", `{"id":1.0}`, "", `{"id":1.0}`},
		{"id booleano resta", `{"id":true}`, "", `{"id":true}`},
		{"id uguale all'utente", `{"id":"collaudo"}`, "collaudo", `{"id":"__UTENTE__"}`},
		{"collection_id zero resta", `{"collection_id":0,"row_id":12}`, "", `{"collection_id":0,"row_id":"__ID__"}`},
		{"user_id group_id collection_id", `{"user_id":3,"group_id":4,"collection_id":7}`, "", `{"user_id":"__ID__","group_id":"__ID__","collection_id":"__ID__"}`},
		{"chiavi non previste restano", `{"status":5,"note":"2026-09-18 06:39:09","code":"1-2-0"}`, "", `{"status":5,"note":"2026-09-18 06:39:09","code":"1-2-0"}`},
		{"guid", `{"guid":"1-2-0"}`, "", `{"guid":"__GUID__"}`},
		{"guid con a capo finale", `{"guid":"1-2-0\n"}`, "", `{"guid":"__GUID__"}`},
		{"guid non n-n-n resta", `{"guid":"collaudo"}`, "", `{"guid":"collaudo"}`},
		{"guid con testo prima resta", `{"guid":"x1-2-0"}`, "", `{"guid":"x1-2-0"}`},
		{"guid con testo dopo resta", `{"guid":"1-2-0x"}`, "", `{"guid":"1-2-0x"}`},
		{"utente solo se uguale, anche annidato", `{"collaudo":[{"name":"collaudo"},["collaudo","collaudo2","xcollaudo","Collaudo"]]}`, "collaudo",
			`{"collaudo":[{"name":"__UTENTE__"},["__UTENTE__","collaudo2","xcollaudo","Collaudo"]]}`},
		{"utente vuoto non sostituisce", `{"name":"collaudo","note":""}`, "", `{"name":"collaudo","note":""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := mustDecode(t, c.in)
			before := mustSerialize(t, in)
			got := mustSerialize(t, Normalize(in, c.user))
			if want := mustSerialize(t, mustDecode(t, c.want)); !bytes.Equal(got, want) {
				t.Errorf("Normalize(%s):\n--- atteso\n%s--- ottenuto\n%s", c.in, want, got)
			}
			if after := mustSerialize(t, in); !bytes.Equal(after, before) {
				t.Errorf("Normalize ha cambiato l'input:\n--- prima\n%s--- dopo\n%s", before, after)
			}
		})
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	for _, s := range []string{"404 not found", "{} {}", ""} {
		if v, err := Decode([]byte(s)); err == nil {
			t.Errorf("Decode(%q) = %v senza errore: json.loads lo rifiuta", s, v)
		}
	}
	mustDecode(t, " {} \n")
	const big = "9007199254740993" // 2^53+1: con float64 diventerebbe ...992
	if got := string(mustSerialize(t, mustDecode(t, big))); got != big+"\n" {
		t.Errorf("Serialize(Decode(%s)) = %q", big, got)
	}
}

// TestSerializeEscapes fissa l'uscita di serializza() su cio' che i golden di
// oggi non contengono: UTF-8 grezzo, U+2028 e U+2029 e caratteri di controllo
// con escape, < & > grezzi.
func TestSerializeEscapes(t *testing.T) {
	const want = "[\n  \"\u00e8\\u2028\\u2029\\u0001<&>\"\n]\n"
	if got := string(mustSerialize(t, mustDecode(t, `["\u00e8\u2028\u2029\u0001<&>"]`))); got != want {
		t.Errorf("Serialize = %q, atteso %q", got, want)
	}
}

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("Decode(%q): %v", s, err)
	}
	return v
}

func mustSerialize(t *testing.T, v any) []byte {
	t.Helper()
	b, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return b
}
