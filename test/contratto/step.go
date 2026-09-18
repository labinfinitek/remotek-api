package contratto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// requestTimeout e' il timeout del client RustDesk e del registratore.
const requestTimeout = 12 * time.Second

// maxResponseBytes limita la lettura di una risposta: le buste di oggi stanno
// sotto il KiB, una risposta oltre 4 MiB e' un errore da guardare.
const maxResponseBytes = 4 << 20

// NewClient restituisce il client HTTP della riesecuzione, impostato come
// quello del registratore: timeout di 12 secondi, nessun proxy preso
// dall'ambiente, nessuna decompressione e nessun redirect seguito (un 3xx e'
// una risposta da confrontare). Differenze sulla rete, indifferenti per
// l'API: niente Accept-Encoding (urllib manda identity) e connessione tenuta
// aperta (urllib manda Connection: close).
func NewClient() *http.Client {
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: &http.Transport{Proxy: nil, DisableCompression: true},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// RunStep manda con c la richiesta del passo s a baseURL e confronta la busta
// della risposta col golden: restituisce "" se coincidono, altrimenti la
// Diff. Come il registratore, costruisce la busta col nome utente di vars e
// solo dopo salva in vars i valori indicati da s.Meta.Extract, presi dal
// corpo grezzo: vars viene aggiornata per i passi successivi e non puo'
// essere nil se il passo estrae. Una risposta compressa o oltre 4
// MiB e' un errore: il registratore apre il gzip, ma l'API non comprime e
// decomprimere qui nasconderebbe un cambiamento.
func RunStep(ctx context.Context, c *http.Client, baseURL string, s *Step, vars map[string]string) (string, error) {
	req, err := s.Request.Build(ctx, baseURL, vars)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("richiesta: %w", err)
	}
	defer resp.Body.Close()
	if enc := resp.Header.Get("Content-Encoding"); enc != "" {
		return "", fmt.Errorf("risposta compressa non prevista (Content-Encoding %s)", enc)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("lettura della risposta: %w", err)
	}
	if len(data) > maxResponseBytes {
		return "", fmt.Errorf("risposta oltre %d byte", maxResponseBytes)
	}
	env := Envelope(resp.StatusCode, resp.Header.Get("Content-Type"), data, s.NestedKeys, vars["utente"])
	extract(data, s.Meta.Extract, vars)
	got, err := Serialize(env)
	if err != nil {
		return "", err
	}
	return compareEnvelope(s, got)
}

// extract salva in vars, per ogni variabile -> chiave di keys, il valore
// della chiave nel corpo data se e' una stringa non vuota, come il giro del
// registratore; un corpo che non e' un oggetto JSON non estrae nulla. Il
// registratore guarda il corpo dopo aver aperto le chiavi annidate: cambia
// solo per una variabile presa da una chiave annidata, che la tabella non ha.
func extract(data []byte, keys, vars map[string]string) {
	body, _ := Decode(data) // se non e' JSON body e' nil: niente da estrarre
	obj, _ := body.(map[string]any)
	for variable, key := range keys {
		if v, _ := obj[key].(string); v != "" {
			vars[variable] = v
		}
	}
}

// compareEnvelope confronta la busta serializzata got col golden di s: al
// byte per "esatto"; per "forma" dopo EnvelopeShape su entrambe, rilette dai
// byte cosi' che passino dallo stesso codice con gli stessi tipi.
func compareEnvelope(s *Step, got []byte) (string, error) {
	switch s.Meta.Compare {
	case compareExact:
		return Diff(s.Golden, got), nil
	case compareShape:
		wantShape, errWant := shapeOf(s.Golden)
		gotShape, errGot := shapeOf(got)
		if err := errors.Join(errWant, errGot); err != nil {
			return "", fmt.Errorf("confronto forma: %w", err)
		}
		return Diff(wantShape, gotShape), nil
	}
	return "", fmt.Errorf("confronto %q sconosciuto", s.Meta.Compare)
}

// shapeOf rilegge la busta serializzata b e ne restituisce la forma
// serializzata.
func shapeOf(b []byte) ([]byte, error) {
	v, err := Decode(b)
	if err != nil {
		return nil, err
	}
	env, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("la busta non e' un oggetto")
	}
	return Serialize(EnvelopeShape(env))
}
