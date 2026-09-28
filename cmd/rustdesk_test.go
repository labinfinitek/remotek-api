package main

import (
	"strings"
	"testing"
)

// TestAvvisiIndirizzi prova sul binario vero che l'avvio col file
// d'esempio, dove id-server e api-server sono vuoti, scriva un warn per
// ciascuno, e nessuno quando le variabili RUSTDESK_API_RUSTDESK_* li
// impostano. relay-server vuoto non ha warn: il client prende il relay da
// hbbs.
func TestAvvisiIndirizzi(t *testing.T) {
	const (
		idVuoto  = "[WARN] rustdesk.id-server vuoto: "
		apiVuoto = "[WARN] rustdesk.api-server vuoto: "
	)
	for _, tc := range []struct {
		caso, id, api string
		want          []string
	}{
		{"file d'esempio", "", "", []string{idVuoto, apiVuoto}},
		{"solo id-server", "hbbs.example.com:21116", "", []string{apiVuoto}},
		{"solo api-server", "", "https://api.example.com", []string{idVuoto}},
		{"entrambi", "hbbs.example.com:21116", "https://api.example.com", nil},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			t.Setenv("RUSTDESK_API_RUSTDESK_ID_SERVER", tc.id) // vuota: per viper non c'e'
			t.Setenv("RUSTDESK_API_RUSTDESK_API_SERVER", tc.api)
			codice, out := esegui(t, sandbox(t))
			if codice != 0 {
				t.Fatalf("avvio: codice %d\n%s", codice, out)
			}
			for _, riga := range []string{idVuoto, apiVuoto} {
				n, want := strings.Count(out, riga), 0
				for _, w := range tc.want {
					if w == riga {
						want = 1
					}
				}
				if n != want {
					t.Errorf("%d righe %q, attese %d\n%s", n, riga, want, out)
				}
			}
			if strings.Contains(out, "relay-server vuoto") {
				t.Errorf("warn per relay-server vuoto\n%s", out)
			}
		})
	}
}
