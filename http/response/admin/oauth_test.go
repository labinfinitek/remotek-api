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

// valorizza da' a ogni campo di v un valore diverso dallo zero, anche nei
// campi delle struct annidate e dietro i puntatori: un omitempty non nasconde
// niente, qualunque sia il tipo del campo.
func valorizza(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				valorizza(v.Field(i))
			}
		}
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		valorizza(v.Elem())
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		valorizza(v.Index(0))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
	}
}

// TestOauthPannelloAllineato tiene OauthPannello, che copia i campi di
// model.Oauth, allineato al modello: le chiavi JSON sono quelle di
// model.Oauth senza client_secret e con has_client_secret. Un campo nuovo
// del modello che non arriva al pannello fa fallire il test. Il provider di
// prova ha ogni campo valorizzato da valorizza, perche' un omitempty non
// nasconda niente.
func TestOauthPannelloAllineato(t *testing.T) {
	o := &model.Oauth{}
	valorizza(reflect.ValueOf(o).Elem())

	modello := slices.DeleteFunc(chiavi(t, o), func(c string) bool { return c == "client_secret" })
	modello = append(modello, "has_client_secret")
	slices.Sort(modello)
	if pannello := chiavi(t, NuovoOauthPannello(o)); !slices.Equal(pannello, modello) {
		t.Errorf("chiavi JSON di OauthPannello:\n got  %v\n want %v", pannello, modello)
	}
}
