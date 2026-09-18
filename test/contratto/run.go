package contratto

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
)

// Tester e' la parte di *testing.T che usa Run: *testing.T la soddisfa come
// Tester[*testing.T], e i test del pacchetto con un finto che raccoglie i
// fallimenti di Run senza far fallire il test. Il parametro e' il tipo
// stesso perche' Run passa ai sottotest un valore dello stesso tipo.
type Tester[T any] interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Run(name string, f func(T)) bool
}

// Options dice a Run dove sta l'API, dove sono i golden e cosa rieseguire.
type Options struct {
	// BaseURL e' l'indirizzo dell'API senza percorso, come http://host:porta.
	BaseURL string
	// Golden e' la cartella client-<versione>: scenario.json e un passo per
	// cartella.
	Golden fs.FS
	// Groups sono i gruppi di scenario.json da rieseguire: ogni loro passo
	// deve avere il golden.
	Groups []string
	// Vars e' il contesto iniziale dello scenario (utente, password): Run
	// lavora su una copia.
	Vars map[string]string
}

// Run riesegue verso o.BaseURL i passi dei gruppi o.Groups, nell'ordine di
// scenario.json e uno per sottotest, e li confronta coi golden. Fail-closed:
// scenario illeggibile, gruppo che lo scenario non ha o nessun passo fermano
// tutto; golden mancante o non valido, segnaposto senza valore o errore di
// rete fermano il passo; una risposta diversa dal golden fa fallire il passo
// e si prosegue. Niente sottotest in parallelo: un passo usa lo stato
// dell'API e le variabili lasciati dai precedenti.
func Run[T Tester[T]](t T, o Options) {
	t.Helper()
	var steps []ScenarioStep
	sc, err := LoadScenario(o.Golden)
	if err == nil {
		steps, err = selectSteps(sc, o.Groups)
	}
	if err != nil {
		t.Fatalf("scenario: %v", err)
		return
	}
	c := NewClient()
	defer c.CloseIdleConnections()
	vars := map[string]string{}
	maps.Copy(vars, o.Vars)
	for _, st := range steps {
		t.Run(st.Name, func(t T) {
			s, err := LoadStep(o.Golden, st.Name)
			if err != nil {
				t.Fatalf("golden mancante o non valido in un gruppo attivo: %v", err)
				return
			}
			switch d, err := RunStep(t.Context(), c, o.BaseURL, s, vars); {
			case err != nil:
				t.Fatalf("%v", err)
			case d != "":
				t.Errorf("%s", d)
			}
		})
	}
}

// selectSteps restituisce i passi di sc dei gruppi groups, nell'ordine dello
// scenario. Un gruppo che lo scenario non ha e nessun passo scelto sono
// errori: un nome sbagliato non deve dare un verde che non ha provato nulla.
func selectSteps(sc Scenario, groups []string) ([]ScenarioStep, error) {
	for _, g := range groups {
		if !slices.ContainsFunc(sc.Steps, func(s ScenarioStep) bool { return s.Group == g }) {
			return nil, fmt.Errorf("gruppo %q assente dallo scenario", g)
		}
	}
	var steps []ScenarioStep
	for _, s := range sc.Steps {
		if slices.Contains(groups, s.Group) {
			steps = append(steps, s)
		}
	}
	if len(steps) == 0 {
		return nil, errors.New("nessun passo da rieseguire")
	}
	return steps, nil
}
