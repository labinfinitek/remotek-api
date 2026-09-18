package contratto

import (
	"bytes"
	"testing"
)

func TestShape(t *testing.T) {
	shape := func(s string) string { return string(mustSerialize(t, Shape(mustDecode(t, s)))) }
	want := string(mustSerialize(t, mustDecode(t, `{"code":"number","data":"string","list":["number","number","string","bool","null",{"ok":"bool"}],"obj":{}}`)))
	for in, same := range map[string]bool{
		`{"code":0,"data":"2.7\n","list":[1,-2.5,"x",true,null,{"ok":false}],"obj":{}}`:      true,
		`{"code":7,"data":"","list":[2,0,"y",false,null,{"ok":true}],"obj":{}}`:              true,
		`{"code":0,"data":null,"list":[1,-2.5,"x",true,null,{"ok":false}],"obj":{}}`:         false,
		`{"code":0,"data":"2.7\n","list":[1,-2.5,"x",true,null,{"ok":false}],"obj":{"a":1}}`: false,
	} {
		if got := shape(in); (got == want) != same {
			t.Errorf("Shape(%s) = %s; uguale a %s atteso %v", in, got, want, same)
		}
	}
	if a, b := Shape(1.5), Shape(3); a != "number" || b != "int" { // float64 e' un numero JSON, int no
		t.Errorf("Shape(1.5), Shape(3) = %v, %v", a, b)
	}
}

// TestEnvelopeShape prova che stato, content_type e corpo_tipo restano valori,
// che corpo e corpo_annidato passano da Shape e che env resta intatta.
func TestEnvelopeShape(t *testing.T) {
	env := Envelope(401, ctJSON, []byte(`{"data":"{\"a\":1}","error":"x"}`), []string{"data"}, "")
	before := mustSerialize(t, env)
	const want = `{"content_type":"application/json; charset=utf-8","corpo":{"data":"string","error":"string"},"corpo_annidato":{"data":{"a":"number"}},"corpo_tipo":"json","stato":401}`
	if d := Diff(mustSerialize(t, mustDecode(t, want)), mustSerialize(t, EnvelopeShape(env))); d != "" {
		t.Error(d)
	}
	if after := mustSerialize(t, env); !bytes.Equal(after, before) {
		t.Errorf("EnvelopeShape ha cambiato la busta:\n--- prima\n%s--- dopo\n%s", before, after)
	}
}

func TestDiff(t *testing.T) {
	if d := Diff([]byte("a\nb\n"), []byte("a\nb\n")); d != "" {
		t.Errorf("Diff di due buste uguali = %q", d)
	}
	for _, c := range [][3]string{{"a\n", "x\n", "1"}, {"a\nb\nc\n", "a\nb\nx\n", "3"}, {"a\nb\n", "a\nb\nc\n", "3"}, {"a", "a\nb", "2"}} {
		want := "prima differenza alla riga " + c[2] + "\n--- atteso\n" + c[0] + "--- ottenuto\n" + c[1]
		if d := Diff([]byte(c[0]), []byte(c[1])); d != want {
			t.Errorf("Diff(%q, %q) = %q, atteso %q", c[0], c[1], d, want)
		}
	}
}
