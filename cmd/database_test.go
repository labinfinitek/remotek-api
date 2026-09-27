package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
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
// errore e l'API sbagliava alla prima scrittura. Ogni caso gira col database
// in delete, come quello in esercizio prima di ADR-0007, e in WAL: l'errore
// arriva dall'apertura (il passaggio a WAL scrive) o dalla scrittura di
// prova, e il messaggio e' lo stesso. Da root i permessi non contano: i casi
// si saltano, e in CI il runner non e' root.
func TestDatabaseNonScrivibile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("da root il file e la cartella restano scrivibili")
	}
	for _, p := range []struct {
		caso, cosa   string
		ostacolo     func(data string) error
		restanoInWAL []string
	}{
		// Col database in WAL il figlio lo apre in sola lettura, ci crea -wal e
		// -shm (la cartella si scrive) e non puo' fare il checkpoint che alla
		// chiusura li cancellerebbe.
		{"file 0444", "il file e' in sola lettura per l'utente del processo", func(data string) error {
			return os.Chmod(filepath.Join(data, "rustdeskapi.db"), 0o444)
		}, []string{"rustdeskapi.db-shm", "rustdeskapi.db-wal"}},
		{"cartella 0555", "la cartella non e' scrivibile e SQLite non ci crea il journal", func(data string) error {
			return os.Chmod(data, 0o555)
		}, nil},
	} {
		for _, modo := range []string{"delete", "wal"} {
			t.Run(p.caso+" in "+modo, func(t *testing.T) {
				dir := sandbox(t)
				data := filepath.Join(dir, "data")
				if codice, out := esegui(t, dir); codice != 0 {
					t.Fatalf("primo avvio: codice %d\n%s", codice, out)
				}
				var restano []string
				if modo == "delete" {
					inDelete(t, dir)
				} else {
					restano = p.restanoInWAL
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
				// Un database in WAL si legge solo potendo creare -wal e -shm
				// nella cartella.
				if err := os.Chmod(data, 0o700); err != nil {
					t.Fatal(err)
				}
				senzaTraccia(t, dir, restano...)
			})
		}
	}
}

// TestDatabaseWAL prova sul binario vero ADR-0007 su un database in delete,
// come quello in esercizio: al primo avvio passa a WAL senza perdere righe.
// Dopo la migrazione nessuna tabella ha chiavi esterne: la migrazione non ne
// crea (DisableForeignKeyConstraintWhenMigrating), e con foreign_keys acceso
// una chiave esterna cambierebbe cosa si puo' scrivere e cancellare.
func TestDatabaseWAL(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	inDelete(t, dir)
	db := apriDB(t, dir)
	if err := db.Create(&model.Tag{Name: "scritto-in-delete", UserId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("avvio sul database in delete: codice %d\n%s", codice, out)
	}
	var modo string
	if err := db.Raw("PRAGMA journal_mode").Scan(&modo).Error; err != nil || modo != "wal" {
		t.Errorf("journal_mode %q (%v), atteso wal", modo, err)
	}
	if err := db.Where("name = ?", "scritto-in-delete").First(&model.Tag{}).Error; err != nil {
		t.Errorf("tag scritto col database in delete: %v", err)
	}
	if u := admin(t, db); u.Username != "admin" {
		t.Errorf("l'utente 1 e' %q, atteso admin", u.Username)
	}
	var tabelle []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'table'").Scan(&tabelle).Error; err != nil || !slices.Contains(tabelle, "users") {
		t.Fatalf("tabelle dopo la migrazione: %v (%v)", tabelle, err)
	}
	for _, tabella := range tabelle {
		var chiavi int64
		if err := db.Raw("SELECT count(*) FROM pragma_foreign_key_list(?)", tabella).Scan(&chiavi).Error; err != nil || chiavi != 0 {
			t.Errorf("chiavi esterne della tabella %s: %d (%v), attese nessuna", tabella, chiavi, err)
		}
	}
}

// TestDatabaseDanneggiato prova sul binario vero che un database che SQLite
// dice danneggiato fermi l'avvio con codice 1 e un messaggio col percorso
// assoluto che dice di ripristinare l'ultimo backup: una riga che viola il
// NOT NULL scritto dopo nello schema, che si legge e che integrity_check
// trova, e un file che non e' un database, che SQLite rifiuta gia'
// all'apertura.
func TestDatabaseDanneggiato(t *testing.T) {
	for _, p := range []struct {
		caso, cosa string
		danno      func(t *testing.T, dir string)
	}{
		{"NOT NULL violato", "integrity_check: NULL value in danneggiata.x", func(t *testing.T, dir string) {
			t.Helper()
			if codice, out := esegui(t, dir); codice != 0 {
				t.Fatalf("primo avvio: codice %d\n%s", codice, out)
			}
			// Un solo Exec: writable_schema vale per la connessione.
			err := apriDB(t, dir).Exec("CREATE TABLE danneggiata (x); INSERT INTO danneggiata VALUES (NULL); " +
				"PRAGMA writable_schema = ON; " +
				"UPDATE sqlite_master SET sql = 'CREATE TABLE danneggiata (x NOT NULL)' WHERE name = 'danneggiata'").Error
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"non e' un database", "file is not a database", func(t *testing.T, dir string) {
			t.Helper()
			testo := []byte(strings.Repeat("non sono un database SQLite\n", 200))
			if err := os.WriteFile(filepath.Join(dir, "data", "rustdeskapi.db"), testo, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(p.caso, func(t *testing.T) {
			dir := sandbox(t)
			p.danno(t, dir)
			codice, out := esegui(t, dir)
			if codice != 1 {
				t.Errorf("codice %d, atteso 1\n%s", codice, out)
			}
			registro, err := os.ReadFile(filepath.Join(dir, "runtime", "log.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, atteso := range []string{
				"database " + filepath.Join(dir, "data", "rustdeskapi.db") + " danneggiato, l'API non parte",
				p.cosa,
				"Ripristina l'ultimo backup",
			} {
				if !strings.Contains(string(registro), atteso) {
					t.Errorf("il log non contiene %q:\n%s", atteso, registro)
				}
			}
		})
	}
}

// TestDatabaseScrivibile prova sul binario vero che la scrittura di prova
// non lasci traccia, sul database nuovo e su quello che c'e' gia': ne' la
// tabella ne' il journal, e alla chiusura neanche -wal e -shm.
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

// senzaTraccia controlla che in data/ ci siano solo il database, la password
// iniziale e i file restano, e nel database nessuna tabella della scrittura
// di prova. -wal e -shm non ci sono se il figlio ha chiuso il database e la
// sua era l'ultima connessione.
func senzaTraccia(t *testing.T, dir string, restano ...string) {
	t.Helper()
	voci, err := os.ReadDir(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	ammessi := append([]string{"rustdeskapi.db", "admin-password.txt"}, restano...)
	for _, v := range voci {
		if n := v.Name(); !slices.Contains(ammessi, n) {
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

// inDelete riporta il database del figlio in journal_mode delete, come
// quello in esercizio prima di ADR-0007.
func inDelete(t *testing.T, dir string) {
	t.Helper()
	var modo string
	if err := apriDB(t, dir).Raw("PRAGMA journal_mode = DELETE").Scan(&modo).Error; err != nil || modo != "delete" {
		t.Fatalf("journal_mode %q (%v), atteso delete", modo, err)
	}
}
