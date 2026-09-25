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

// TestDatabaseNonScrivibile prova sul binario vero che un database che si
// apre ma non si scrive, come quello lasciato da un'immagine che girava come
// root, fermi l'avvio con codice 1 e un messaggio che dice cosa non si
// scrive e come rimediare. Prima SQLite lo apriva in sola lettura senza
// errore e l'API sbagliava alla prima scrittura. Da root i permessi non
// contano: i casi si saltano, e in CI il runner non e' root.
func TestDatabaseNonScrivibile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("da root il file e la cartella restano scrivibili")
	}
	for _, p := range []struct {
		caso, cosa string
		ostacolo   func(data string) error
	}{
		{"file 0444", "il file e' in sola lettura per l'utente del processo", func(data string) error {
			return os.Chmod(filepath.Join(data, "rustdeskapi.db"), 0o444)
		}},
		{"cartella 0555", "la cartella non e' scrivibile e SQLite non ci crea il journal", func(data string) error {
			return os.Chmod(data, 0o555)
		}},
	} {
		t.Run(p.caso, func(t *testing.T) {
			dir := sandbox(t)
			data := filepath.Join(dir, "data")
			if codice, out := esegui(t, dir); codice != 0 {
				t.Fatalf("primo avvio: codice %d\n%s", codice, out)
			}
			if err := p.ostacolo(data); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(data, 0o700) })
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
				"database " + filepath.Join(data, "rustdeskapi.db") + " non scrivibile, l'API non parte",
				p.cosa,
				"la cartella " + data + " devono essere dell'utente del processo",
				"chown -R 10001:10001 della cartella dati",
			} {
				if !strings.Contains(string(registro), atteso) {
					t.Errorf("il log non contiene %q:\n%s", atteso, registro)
				}
			}
			senzaTraccia(t, dir)
		})
	}
}

// TestDatabaseScrivibile prova sul binario vero che la scrittura di prova
// non lasci traccia, sul database nuovo e su quello che c'e' gia': ne' la
// tabella ne' il journal.
func TestDatabaseScrivibile(t *testing.T) {
	dir := sandbox(t)
	for _, avvio := range []string{"primo avvio", "secondo avvio"} {
		if codice, out := esegui(t, dir); codice != 0 {
			t.Fatalf("%s: codice %d, atteso 0\n%s", avvio, codice, out)
		}
		senzaTraccia(t, dir)
	}
	if u := admin(t, apriDB(t, dir)); u.Username != "admin" {
		t.Errorf("l'utente 1 e' %q, atteso admin", u.Username)
	}
}

// senzaTraccia controlla che in data/ ci siano solo il database e la password
// iniziale, e nel database nessuna tabella della scrittura di prova.
func senzaTraccia(t *testing.T, dir string) {
	t.Helper()
	voci, err := os.ReadDir(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range voci {
		if n := v.Name(); n != "rustdeskapi.db" && n != "admin-password.txt" {
			t.Errorf("in data/ e' rimasto %s", n)
		}
	}
	var n int64
	if err := apriDB(t, dir).Raw("SELECT count(*) FROM sqlite_master WHERE name = 'remotek_prova_scrittura'").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("la tabella della scrittura di prova e' rimasta nel database")
	}
}
