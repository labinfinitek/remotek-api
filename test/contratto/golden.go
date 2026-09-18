package contratto

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
)

// Valori di confronto in meta.json: "forma" guarda solo i tipi del corpo.
const compareExact, compareShape = "esatto", "forma"

// Scenario e' scenario.json: i passi nell'ordine del registratore, anche
// quelli non ancora registrati.
type Scenario struct {
	Client string         `json:"client"`
	Steps  []ScenarioStep `json:"passi"`
}

// ScenarioStep e' un passo dello scenario: cartella del golden e gruppo.
type ScenarioStep struct {
	Name  string `json:"nome"`
	Group string `json:"gruppo"`
}

// Request e' richiesta.json: la richiesta del client con i segnaposto al
// posto dei valori veri; Query sono coppie nell'ordine del client.
type Request struct {
	Method    string            `json:"metodo"`
	Path      string            `json:"percorso"`
	Query     [][]string        `json:"query"`
	Headers   map[string]string `json:"intestazioni"`
	Body      json.RawMessage   `json:"corpo"`
	EmptyBody bool              `json:"corpo_vuoto"`
}

// Meta e' la parte di meta.json che serve alla riesecuzione: come si
// confronta e quali valori della risposta salvare nel contesto dello
// scenario (Extract, variabile -> chiave; nil se il file non ha estrai).
type Meta struct {
	Compare string            `json:"confronto"`
	Extract map[string]string `json:"estrai"`
}

// Step e' un passo pronto da rieseguire: Golden sono i byte di
// risposta.golden.json, NestedKeys le chiavi di corpo_annidato, cioe' quelle
// che il registratore ha aperto con OpenNested.
type Step struct {
	Name       string
	Request    Request
	Meta       Meta
	Golden     []byte
	NestedKeys []string
}

// LoadScenario legge scenario.json dalla radice di fsys.
func LoadScenario(fsys fs.FS) (Scenario, error) {
	var sc Scenario
	err := readJSON(fsys, "scenario.json", &sc)
	return sc, err
}

// LoadStep legge da fsys i tre file del passo name e li valida: metodo,
// percorso che inizia con "/", coppie della query, confronto "esatto" o
// "forma", golden che e' un oggetto con corpo_annidato oggetto se c'e'. Se
// un file manca, errors.Is(err, fs.ErrNotExist) vale.
func LoadStep(fsys fs.FS, name string) (*Step, error) {
	s := &Step{Name: name}
	if err := readJSON(fsys, path.Join(name, "richiesta.json"), &s.Request); err != nil {
		return nil, err
	}
	if err := s.Request.validate(); err != nil {
		return nil, fmt.Errorf("%s/richiesta.json: %w", name, err)
	}
	if err := readJSON(fsys, path.Join(name, "meta.json"), &s.Meta); err != nil {
		return nil, err
	}
	if s.Meta.Compare != compareExact && s.Meta.Compare != compareShape {
		return nil, fmt.Errorf("%s/meta.json: confronto %q sconosciuto", name, s.Meta.Compare)
	}
	golden, err := fs.ReadFile(fsys, path.Join(name, "risposta.golden.json"))
	if err != nil {
		return nil, fmt.Errorf("lettura: %w", err)
	}
	v, err := Decode(golden)
	if err != nil {
		return nil, fmt.Errorf("%s/risposta.golden.json: %w", name, err)
	}
	// il registratore scrive sempre un oggetto, e corpo_annidato solo per le
	// chiavi che ha aperto: un'altra forma e' un golden rotto
	env, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s/risposta.golden.json: la busta non e' un oggetto", name)
	}
	nested, ok := env["corpo_annidato"].(map[string]any)
	if _, found := env["corpo_annidato"]; found && !ok {
		return nil, fmt.Errorf("%s/risposta.golden.json: corpo_annidato non e' un oggetto", name)
	}
	s.Golden, s.NestedKeys = golden, slices.Sorted(maps.Keys(nested))
	return s, nil
}

// validate controlla cio' che serve per costruire la richiesta: metodo,
// percorso che inizia con "/" e coppie della query di due elementi.
func (r Request) validate() error {
	if r.Method == "" {
		return errors.New("metodo vuoto")
	}
	if !strings.HasPrefix(r.Path, "/") {
		return fmt.Errorf("percorso %q senza / iniziale", r.Path)
	}
	for _, pair := range r.Query {
		if len(pair) != 2 {
			return fmt.Errorf("coppia della query %q: %d elementi, attesi 2", pair, len(pair))
		}
	}
	return nil
}

// readJSON decodifica in v il file name di fsys. Solo io/fs: un fs.FS non
// accetta percorsi che escono dalla sua radice.
func readJSON(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("lettura: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
