package contratto

import (
	"maps"
	"regexp"
	"strings"
	"unicode/utf8"
)

// timeInText e' RE_TEMPO_NEL_TESTO del registratore: data e ora ovunque nel testo.
var timeInText = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

// Valori di corpo_tipo nella busta, gli stessi del registratore.
const bodyEmpty, bodyText, bodyJSON = "vuoto", "testo", "json"

// OpenNested apre, come apri_json_annidato() del registratore, le keys di un
// oggetto body con JSON in una stringa (data in GET /api/ab): in una copia di
// body diventano "__JSON:<chiave>__" e il valore aperto e normalizzato, con
// tag_colors avvolto in {"__JSON__": ...} se anch'esso JSON, va nella mappa.
func OpenNested(body any, keys []string, user string) (any, map[string]any) {
	nested := map[string]any{}
	obj, ok := body.(map[string]any)
	if !ok {
		return body, nested
	}
	out := maps.Clone(obj)
	for _, key := range keys {
		s, _ := out[key].(string) // assente o non stringa: "", che Decode rifiuta
		inner, err := Decode([]byte(s))
		if err != nil {
			continue
		}
		m, _ := inner.(map[string]any) // se inner non e' un oggetto m e' nil e colors ""
		colors, _ := m["tag_colors"].(string)
		if v, err := Decode([]byte(colors)); err == nil {
			m["tag_colors"] = map[string]any{"__JSON__": v}
		}
		nested[key] = Normalize(inner, user)
		out[key] = "__JSON:" + key + "__"
	}
	return out, nested
}

// Envelope costruisce la busta di una risposta (risposta.golden.json) come
// busta_risposta() del registratore: stato, content_type e corpo_tipo "vuoto"
// senza byte, "testo" se non sono un solo valore JSON (date sostituite da
// PlaceholderTime) o "json" (corpo normalizzato, corpo_annidato se non vuoto).
func Envelope(status int, contentType string, data []byte, nestedKeys []string, user string) map[string]any {
	env := map[string]any{"stato": status, "content_type": contentType}
	if len(data) == 0 {
		env["corpo_tipo"], env["corpo"] = bodyEmpty, nil
		return env
	}
	// come decode("utf-8", errors="replace"), ma qui una serie di byte non
	// validi consecutivi diventa un solo U+FFFD (limite nel README)
	text := strings.ToValidUTF8(string(data), string(utf8.RuneError))
	body, err := Decode([]byte(text))
	if err != nil {
		env["corpo_tipo"], env["corpo"] = bodyText, timeInText.ReplaceAllLiteralString(text, PlaceholderTime)
		return env
	}
	body, nested := OpenNested(body, nestedKeys, user)
	env["corpo_tipo"], env["corpo"] = bodyJSON, Normalize(body, user)
	if len(nested) > 0 {
		env["corpo_annidato"] = nested
	}
	return env
}
