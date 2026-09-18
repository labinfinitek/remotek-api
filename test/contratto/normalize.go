package contratto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Segnaposto scritti nei golden al posto dei valori volatili: gli stessi del
// registratore, solo ASCII e senza < > &.
const (
	// PlaceholderToken sostituisce il valore di access_token.
	PlaceholderToken = "__TOKEN__"
	// PlaceholderTime sostituisce le date di created_at e updated_at.
	PlaceholderTime = "__TIMESTAMP__"
	// PlaceholderID sostituisce gli id interi positivi (id di riga del DB).
	PlaceholderID = "__ID__"
	// PlaceholderGUID sostituisce i guid di rubrica nella forma n-n-n.
	PlaceholderGUID = "__GUID__"
	// PlaceholderUser sostituisce ogni stringa uguale al nome dell'utente di collaudo.
	PlaceholderUser = "__UTENTE__"
	// PlaceholderPassword sta nelle richieste al posto della password dell'utente di collaudo.
	PlaceholderPassword = "__PASSWORD__"
)

// Espressioni del registratore (RE_TEMPO e RE_GUID). In guidForm "\n?$"
// riproduce il "$" di Python, che vale anche prima di un a capo finale; in Go
// "$" vale solo a fine testo.
var (
	timeStart = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}`)
	guidForm  = regexp.MustCompile(`^\d+-\d+-\d+\n?$`)
)

// Chiavi i cui valori cambiano a ogni registrazione (CHIAVI_* del registratore).
var (
	tokenKeys = map[string]bool{"access_token": true}
	timeKeys  = map[string]bool{"created_at": true, "updated_at": true}
	idKeys    = map[string]bool{"id": true, "row_id": true, "user_id": true, "collection_id": true, "group_id": true}
	guidKeys  = map[string]bool{"guid": true}
)

// Decode legge un solo valore JSON con i numeri come json.Number, cosi' un
// intero grande resta identico al byte. Come json.loads del registratore
// rifiuta dati dopo il valore: "404 not found" e' testo, non il numero 404. A
// differenza di json.loads rifiuta NaN e Infinity (limiti nel README).
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decodifica del valore JSON: %w", err)
	}
	err := dec.Decode(&struct{}{})
	if err == nil {
		return nil, errors.New("dati dopo il valore JSON")
	}
	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("dati dopo il valore JSON: %w", err)
	}
	return v, nil
}

// Normalize restituisce una copia di v con i valori volatili sostituiti dai
// segnaposto, come normalizza() del registratore: v viene da Decode, l'input
// resta intatto e con user vuoto nessuna stringa diventa PlaceholderUser. Le
// chiavi delle mappe restano come sono.
func Normalize(v any, user string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = normalizeField(k, item, user)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = Normalize(item, user)
		}
		return out
	case string:
		if user != "" && t == user {
			return PlaceholderUser
		}
	}
	return v
}

// normalizeField applica le regole legate al nome della chiave, nello stesso
// ordine di _normalizza_campo(); se nessuna vale, normalizza il valore.
func normalizeField(key string, v any, user string) any {
	s, isString := v.(string)
	n, isNumber := v.(json.Number)
	switch {
	case tokenKeys[key] && isString && s != "":
		return PlaceholderToken
	case timeKeys[key] && isString && timeStart.MatchString(s):
		return PlaceholderTime
	case idKeys[key] && isNumber && positiveInteger(n):
		return PlaceholderID
	case guidKeys[key] && isString && guidForm.MatchString(s):
		return PlaceholderGUID
	}
	return Normalize(v, user)
}

// positiveInteger dice se n e' un intero maggiore di zero, come
// isinstance(v, int) and v > 0 in Python: 1.0 e 1e3 sono float per
// json.loads, quindi restano; lo zero resta (collection_id 0 = rubrica
// personale).
func positiveInteger(n json.Number) bool {
	s := n.String()
	if strings.ContainsAny(s, ".eE") || strings.HasPrefix(s, "-") {
		return false
	}
	return strings.Trim(s, "0") != ""
}

// Serialize scrive v nella forma canonica dei golden, la stessa di
// serializza() del registratore: rientro di due spazi, chiavi ordinate, UTF-8
// senza escape di < > &, U+2028 e U+2029 con escape e a capo finale.
func Serialize(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("serializzazione in forma canonica: %w", err)
	}
	return buf.Bytes(), nil
}
