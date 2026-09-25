package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

// TestPermessiFileDiLog: il file di log contiene nomi utente e indirizzi IP,
// quindi esce 0600 sia quando New lo crea sia quando esisteva gia' 0644.
func TestPermessiFileDiLog(t *testing.T) {
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := t.TempDir()
	vecchio := filepath.Join(dir, "vecchio.txt")
	if err := os.WriteFile(vecchio, []byte("riga di prima\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(vecchio, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(dir, "nuovo.txt"), vecchio} {
		New(&Config{Path: f, Level: "info"})
		info, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s: permessi %v, attesi 0600", filepath.Base(f), info.Mode().Perm())
		}
	}
	righe, err := os.ReadFile(vecchio)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(righe), "riga di prima\n") {
		t.Errorf("il file che c'era ha perso il contenuto: %q", righe)
	}
}

// TestFileDiLogNonApribile: se il file non si apre, il panic dice quale file
// e perche'.
func TestFileDiLogNonApribile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "manca", "log.txt")
	defer func() {
		r := recover()
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic %v, atteso un errore", r)
		}
		for _, s := range []string{f, "no such file or directory"} {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("il panic %q non contiene %q", err, s)
			}
		}
	}()
	New(&Config{Path: f, Level: "info"})
}
