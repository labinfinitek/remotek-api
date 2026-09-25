package main

// Test del primo avvio e dei comandi reset-admin-pwd e reset-pwd sul binario
// vero, in un processo figlio: il codice d'uscita e' parte di cio' che si
// prova, e in questo processo InitGlobal lo puo' chiamare solo TestContract.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/utils"
)

// figlio, nell'ambiente, fa del binario dei test l'API: senza argomenti solo
// il bootstrap (InitGlobal, l'avvio prima di aprire la porta), con argomenti
// quello che fa ./apimain con gli stessi argomenti.
const figlio = "REMOTEK_TEST_FIGLIO"

func TestMain(m *testing.M) {
	if os.Getenv(figlio) == "" {
		os.Exit(m.Run())
	}
	if len(os.Args) == 1 {
		InitGlobal()
		return
	}
	rootCmd.SetArgs(os.Args[1:])
	main()
}

// sandbox prepara, come TestContract, la cartella in cui gira il figlio:
// conf/ e resources/ del repo, data/ e runtime/ vuote.
func sandbox(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, d := range []string{"data", "runtime"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"resources", "conf"} {
		if err := os.Symlink(filepath.Join(root, d), filepath.Join(dir, d)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// esegui fa girare il figlio in dir con args e restituisce il codice d'uscita
// e cio' che ha scritto, incluso il log. Log a trace: la password non deve
// esserci a nessun livello.
func esegui(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), figlio+"=1", "PWD="+dir, "RUSTDESK_API_LOGGER_LEVEL=trace")
	out, err := cmd.CombinedOutput()
	var uscita *exec.ExitError
	if ctx.Err() != nil || (err != nil && !errors.As(err, &uscita)) {
		t.Fatalf("figlio %q: %v", args, errors.Join(err, ctx.Err()))
	}
	return cmd.ProcessState.ExitCode(), string(out)
}

// apriDB apre, senza log, il database sqlite del figlio.
func apriDB(t *testing.T, dir string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "data", "rustdeskapi.db")), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

// admin legge l'utente con id 1, quello che cerca reset-admin-pwd.
func admin(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	u := &model.User{}
	if err := db.First(u, 1).Error; err != nil {
		t.Fatalf("utente 1: %v", err)
	}
	return u
}

// TestPrimoAvvio fa fallire il primo avvio due volte, col file della password
// che non si puo' scrivere e con l'insert di admin rifiutato, e poi lo lascia
// riuscire. Ogni errore deve fermare il processo senza lasciare righe in
// versions, groups e users, cosi' al riavvio il primo avvio si ripete da
// capo; quando riesce, la password iniziale e' nel file e non nel log.
func TestPrimoAvvio(t *testing.T) {
	dir := sandbox(t)
	db := apriDB(t, dir)
	file := filepath.Join(dir, "data", "admin-password.txt")
	righe := func() (n [3]int64) {
		for i, m := range []any{&model.Version{}, &model.Group{}, &model.User{}} {
			if err := db.Model(m).Count(&n[i]).Error; err != nil {
				t.Fatal(err)
			}
		}
		return n
	}
	fallito := func(caso string) {
		t.Helper()
		if codice, out := esegui(t, dir); codice != 1 {
			t.Errorf("%s: codice %d, atteso 1\n%s", caso, codice, out)
		}
		if n := righe(); n != [3]int64{} {
			t.Errorf("%s: righe in versions, groups e users %v, attese nessuna", caso, n)
		}
		if t.Failed() {
			t.FailNow()
		}
	}

	// Al posto del file una cartella non vuota: non si scrive neanche da root.
	if err := os.MkdirAll(filepath.Join(file, "ostacolo"), 0o750); err != nil {
		t.Fatal(err)
	}
	fallito("file non scrivibile")

	if err := os.RemoveAll(file); err != nil {
		t.Fatal(err)
	}
	const rifiuta = "CREATE TRIGGER rifiuta BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'insert rifiutato dal test'); END"
	if err := db.Exec(rifiuta).Error; err != nil {
		t.Fatal(err)
	}
	fallito("insert di admin rifiutato")

	// Un vecchio file leggibile da tutti: quello nuovo deve avere 0600.
	err := errors.Join(db.Exec("DROP TRIGGER rifiuta").Error,
		os.WriteFile(file, []byte("vecchia"), 0o600), os.Chmod(file, 0o644))
	if err != nil {
		t.Fatal(err)
	}
	codice, out := esegui(t, dir)
	if codice != 0 {
		t.Fatalf("avvio senza ostacoli: codice %d\n%s", codice, out)
	}
	if n := righe(); n != [3]int64{1, 2, 1} {
		t.Errorf("righe in versions, groups e users %v, attese [1 2 1]", n)
	}
	pwd, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]{20}$`).Match(pwd) {
		t.Errorf("il file deve contenere solo la password, 20 lettere o cifre: ha %d byte", len(pwd))
	}
	if u := admin(t, db); u.Username != "admin" || !*u.IsAdmin {
		t.Errorf("l'utente 1 e' %q, admin %t: atteso admin", u.Username, *u.IsAdmin)
	} else if ok, err := utils.VerifyPassword(u.Password, string(pwd)); !ok || err != nil {
		t.Errorf("la password del file non e' quella di admin (%v)", err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permessi del file %v, attesi 0600", info.Mode().Perm())
	}
	registro, err := os.ReadFile(filepath.Join(dir, "runtime", "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, string(pwd)) || strings.Contains(string(registro), string(pwd)) {
		t.Error("la password iniziale e' nel log")
	}
	if !strings.Contains(out, "password iniziale di admin in "+file) {
		t.Errorf("il log non dice dov'e' il file:\n%s", out)
	}
}

// TestComandiResetPassword prova reset-admin-pwd e reset-pwd sul binario
// vero: rifiutano password sotto i 15 o sopra i 32 caratteri, contati come
// caratteri e non come byte, ed escono con codice 1 quando rifiutano o
// falliscono, senza toccare la password; una password valida la impostano
// ed escono con 0.
func TestComandiResetPassword(t *testing.T) {
	dir := sandbox(t)
	db := apriDB(t, dir)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	for _, p := range []struct {
		caso   string
		args   []string
		codice int
		uscita string
	}{
		{"14 caratteri, 28 byte", []string{"reset-admin-pwd", strings.Repeat("è", 14)}, 1, "da 15 a 32 caratteri"},
		{"33 caratteri", []string{"reset-admin-pwd", strings.Repeat("a", 33)}, 1, "da 15 a 32 caratteri"},
		{"32 caratteri, 64 byte", []string{"reset-admin-pwd", strings.Repeat("è", 32)}, 0, "reimpostata"},
		{"25 caratteri, 75 byte: bcrypt ne prende 72", []string{"reset-admin-pwd", strings.Repeat("€", 25)}, 1, "non aggiornata"},
		{"utente che non c'e'", []string{"reset-pwd", "2", strings.Repeat("a", 15)}, 1, "utente 2 non trovato"},
		{"userId 0", []string{"reset-pwd", "0", strings.Repeat("a", 15)}, 1, "maggiore di 0"},
		{"15 caratteri", []string{"reset-pwd", "1", strings.Repeat("a", 15)}, 0, "reimpostata"},
	} {
		prima := admin(t, db).Password
		codice, out := esegui(t, dir, p.args...)
		if codice != p.codice || !strings.Contains(out, p.uscita) {
			t.Errorf("%s, %s: codice %d, attesi %d e %q\n%s", p.args[0], p.caso, codice, p.codice, p.uscita, out)
		}
		dopo := admin(t, db).Password
		if p.codice != 0 && dopo != prima {
			t.Errorf("%s, %s: rifiutata o fallita, eppure la password di admin e' cambiata", p.args[0], p.caso)
		}
		if p.codice == 0 {
			if ok, err := utils.VerifyPassword(dopo, p.args[len(p.args)-1]); !ok || err != nil {
				t.Errorf("%s, %s: admin non ha la password nuova (%v)", p.args[0], p.caso, err)
			}
		}
	}
}
