package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDatabaseNonApribile prova sul binario vero che un database che non si
// apre fermi l'avvio con codice 1 e un messaggio nel log che nomina il file
// col percorso assoluto e dice cosa controllare, senza panic. Da root i
// permessi non bastano a chiudere la cartella: al loro posto un file dove
// va la cartella, o una cartella dove va il database, che non si aprono
// neanche da root.
func TestDatabaseNonApribile(t *testing.T) {
	for _, p := range []struct {
		caso       string
		ostacolo   func(t *testing.T, data string)
		soloUtente bool
	}{
		{"data e' un file", func(t *testing.T, data string) {
			t.Helper()
			if err := os.Remove(data); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(data, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"rustdeskapi.db e' una cartella", func(t *testing.T, data string) {
			t.Helper()
			if err := os.MkdirAll(filepath.Join(data, "rustdeskapi.db", "ostacolo"), 0o750); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"data non scrivibile", func(t *testing.T, data string) {
			t.Helper()
			if err := os.Chmod(data, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(data, 0o700) })
		}, true},
	} {
		t.Run(p.caso, func(t *testing.T) {
			if p.soloUtente && os.Geteuid() == 0 {
				t.Skip("da root la cartella resta scrivibile")
			}
			dir := sandbox(t)
			data := filepath.Join(dir, "data")
			p.ostacolo(t, data)
			codice, out := esegui(t, dir)
			if codice != 1 {
				t.Errorf("codice %d, atteso 1\n%s", codice, out)
			}
			if strings.Contains(out, "panic") {
				t.Errorf("panic nell'uscita:\n%s", out)
			}
			registro, err := os.ReadFile(filepath.Join(dir, "runtime", "log.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, atteso := range []string{
				"database " + filepath.Join(data, "rustdeskapi.db") + " non aperto, l'API non parte",
				"la cartella " + data + " esista",
				"scrivibile dall'utente del processo (nell'immagine Docker 10001:10001",
			} {
				if !strings.Contains(string(registro), atteso) {
					t.Errorf("il log non contiene %q:\n%s", atteso, registro)
				}
			}
		})
	}
}

// TestDatabaseCartellaNuova prova sul binario vero che un avvio senza data/
// la crei, con permessi 0700, e arrivi fino ad admin.
func TestDatabaseCartellaNuova(t *testing.T) {
	dir := sandbox(t)
	data := filepath.Join(dir, "data")
	if err := os.Remove(data); err != nil {
		t.Fatal(err)
	}
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("codice %d, atteso 0\n%s", codice, out)
	}
	info, err := os.Stat(data)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("data/ %v, attesa una cartella 0700", info.Mode())
	}
	if u := admin(t, apriDB(t, dir)); u.Username != "admin" {
		t.Errorf("l'utente 1 e' %q, atteso admin", u.Username)
	}
}
