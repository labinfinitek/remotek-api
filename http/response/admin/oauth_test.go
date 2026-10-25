package admin

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// chiavi restituisce le chiavi dell'oggetto JSON di v, in ordine.
func chiavi(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	k := make([]string, 0, len(m))
	for c := range m {
		k = append(k, c)
	}
	slices.Sort(k)
	return k
}

// TestOauthPannelloAllineato tiene OauthPannello, che copia i campi di
// model.Oauth, allineato al modello: le chiavi JSON sono quelle di
// model.Oauth senza client_secret e con has_client_secret. Un campo nuovo
// del modello che non arriva al pannello fa fallire il test. Il provider di
// prova ha ogni campo valorizzato, perche' un omitempty non nasconda niente.
func TestOauthPannelloAllineato(t *testing.T) {
	si := true
	o := &model.Oauth{}
	valori := reflect.ValueOf(o).Elem()
	for i := range valori.NumField() {
		if f := valori.Field(i); f.Kind() == reflect.String {
			f.SetString("x")
		}
	}
	o.Id, o.AutoRegister, o.PkceEnable = 1, &si, &si

	modello := slices.DeleteFunc(chiavi(t, o), func(c string) bool { return c == "client_secret" })
	modello = append(modello, "has_client_secret")
	slices.Sort(modello)
	if pannello := chiavi(t, NuovoOauthPannello(o)); !slices.Equal(pannello, modello) {
		t.Errorf("chiavi JSON di OauthPannello:\n got  %v\n want %v", pannello, modello)
	}
}
