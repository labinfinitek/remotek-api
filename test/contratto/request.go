package contratto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// placeholders sono i segnaposto delle richieste con la variabile dello
// scenario che li sostituisce, nell'ordine di sostituisci() del registratore.
var placeholders = []struct{ text, key string }{
	{PlaceholderToken, "token"},
	{PlaceholderGUID, "guid"},
	{PlaceholderUser, "utente"},
	{PlaceholderPassword, "password"},
}

// substitute sostituisce in s i segnaposto con i valori di vars, come
// sostituisci() del registratore: un segnaposto senza valore e' un errore.
func substitute(s string, vars map[string]string) (string, error) {
	for _, p := range placeholders {
		if !strings.Contains(s, p.text) {
			continue
		}
		if vars[p.key] == "" {
			return "", fmt.Errorf("manca '%s' nel contesto dello scenario", p.key)
		}
		s = strings.ReplaceAll(s, p.text, vars[p.key])
	}
	return s, nil
}

// substituteValue applica substitute alle stringhe di v, dentro mappe (per
// chiave ordinata) e liste, e riscrive v sul posto: v viene da Decode. Le
// chiavi restano.
func substituteValue(v any, vars map[string]string) (any, error) {
	var err error
	switch t := v.(type) {
	case string:
		return substitute(t, vars)
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(t)) {
			if t[k], err = substituteValue(t[k], vars); err != nil {
				return nil, err
			}
		}
	case []any:
		for i, item := range t {
			if t[i], err = substituteValue(item, vars); err != nil {
				return nil, err
			}
		}
	}
	return v, nil
}

// Build costruisce la richiesta verso baseURL come manda() del registratore:
// segnaposto sostituiti da vars nel percorso, nei valori della query, nelle
// intestazioni e nelle stringhe del corpo, in quest'ordine e per chiave
// ordinata, cosi' un valore mancante da' sempre lo stesso errore; escape del
// percorso come quote() e query nell'ordine del file, non riordinata. Tre
// differenze, indifferenti per l'API. Le chiavi degli oggetti del corpo
// escono ordinate, mentre il registratore le manda nell'ordine della sua
// tabella delle richieste, non in quello di richiesta.json: per alcuni passi
// i byte differiscono, ma i controller decodificano il corpo in una struct o
// in una mappa, e il NoRoute non lo legge. U+2028 e U+2029 escono come
// sequenze di escape (json.Encoder lo fa sempre). User-Agent resta quello di
// Go, che l'API non legge.
func (r Request) Build(ctx context.Context, baseURL string, vars map[string]string) (*http.Request, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	p, err := substitute(r.Path, vars)
	if err != nil {
		return nil, fmt.Errorf("percorso: %w", err)
	}
	target := strings.TrimRight(baseURL, "/") + quotePath(p)
	query := make([]string, 0, len(r.Query))
	for _, pair := range r.Query {
		v, err := substitute(pair[1], vars)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", pair[0], err)
		}
		query = append(query, url.QueryEscape(pair[0])+"="+url.QueryEscape(v))
	}
	if len(query) > 0 {
		target += "?" + strings.Join(query, "&")
	}
	header := make(http.Header, len(r.Headers))
	for _, k := range slices.Sorted(maps.Keys(r.Headers)) {
		s, err := substitute(r.Headers[k], vars)
		if err != nil {
			return nil, fmt.Errorf("intestazione %s: %w", k, err)
		}
		header.Set(k, s)
	}
	body, err := r.body(vars)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, target, body)
	if err != nil {
		return nil, fmt.Errorf("richiesta %s %s: %w", r.Method, r.Path, err)
	}
	req.Header = header
	return req, nil
}

// body restituisce il corpo come corpo_in_byte() del registratore: il JSON
// del file con i segnaposto sostituiti, compatto e con < > & grezzi. Senza
// corpo (assente o null) da' http.NoBody se EmptyBody, altrimenti nil. Sulla
// rete sono uguali: net/http manda Content-Length: 0 per POST, PUT e PATCH e
// non per gli altri metodi, mentre il registratore lo manda per ogni
// corpo_vuoto; differiscono quindi GET e DELETE con corpo_vuoto, che nella
// tabella del registratore oggi non ci sono.
func (r Request) body(vars map[string]string) (io.Reader, error) {
	raw := bytes.TrimSpace(r.Body)
	if len(raw) == 0 || string(raw) == "null" {
		if r.EmptyBody {
			return http.NoBody, nil
		}
		return nil, nil
	}
	v, err := Decode(raw) // numeri come json.Number: un session_id vale 2^53+1
	if err != nil {
		return nil, fmt.Errorf("corpo: %w", err)
	}
	if v, err = substituteValue(v, vars); err != nil {
		return nil, fmt.Errorf("corpo: %w", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("corpo: %w", err)
	}
	return bytes.NewReader(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// quotePath fa l'escape del percorso come quote(percorso, safe="/-_.~") del
// registratore: restano solo lettere e cifre ASCII e / - _ . ~, il resto
// diventa %XX. EscapedPath di net/url lascerebbe anche $ & + , : ; = @.
func quotePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		// QueryEscape lascia gli stessi caratteri, ma scrive lo spazio come +
		segments[i] = strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
	}
	return strings.Join(segments, "/")
}
