package contratto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
)

// Shape sostituisce ogni valore di v con il suo tipo JSON ("string", "number",
// "bool", "null") e conserva chiavi e lunghezza delle liste: e' cio' che guarda
// il confronto "forma". Un tipo non JSON da' il suo nome Go, mai uno dei quattro.
func Shape(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = Shape(item)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = Shape(item)
		}
		return out
	case string:
		return "string"
	case json.Number, float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
}

// EnvelopeShape restituisce una copia di env con corpo e corpo_annidato ridotti
// da Shape; le altre chiavi (stato, content_type, corpo_tipo) restano valori.
// Serializzata si confronta al byte come l'esatto, ma ignora i valori del corpo.
func EnvelopeShape(env map[string]any) map[string]any {
	out := maps.Clone(env)
	for _, key := range []string{"corpo", "corpo_annidato"} {
		if v, ok := env[key]; ok {
			out[key] = Shape(v)
		}
	}
	return out
}

// Diff confronta due buste serializzate: "" se sono uguali al byte, altrimenti
// il numero della prima riga diversa e le due buste intere (poche righe).
func Diff(want, got []byte) string {
	if bytes.Equal(want, got) {
		return ""
	}
	w, g := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	i := 0
	for i < min(len(w), len(g)) && w[i] == g[i] {
		i++
	}
	return fmt.Sprintf("prima differenza alla riga %d\n--- atteso\n%s--- ottenuto\n%s", i+1, want, got)
}
