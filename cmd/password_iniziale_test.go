package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScriviPasswordIniziale prova il file della password iniziale di admin
// dove TestPrimoAvvio non arriva: con sqlite la cartella data/ c'e' sempre,
// perche' ci sta il database. La cartella si crea se manca; il file contiene
// solo la password e ha permessi 0600 anche se esisteva con altri; un link
// simbolico al suo posto si sostituisce senza scrivere dove punta.
func TestScriviPasswordIniziale(t *testing.T) {
	t.Chdir(t.TempDir())
	scrivi := func(caso, pwd string) {
		t.Helper()
		if err := scriviPasswordIniziale(pwd); err != nil {
			t.Fatalf("%s: %v", caso, err)
		}
		info, err := os.Lstat(fileAdminPassword)
		if err != nil {
			t.Fatalf("%s: %v", caso, err)
		}
		data, err := os.ReadFile(fileAdminPassword)
		if err != nil {
			t.Fatalf("%s: %v", caso, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || string(data) != pwd {
			t.Errorf("%s: %v con %q, atteso un file 0600 con %q", caso, info.Mode(), data, pwd)
		}
	}
	scrivi("cartella che manca", "prima")

	if err := os.Chmod(fileAdminPassword, 0o644); err != nil {
		t.Fatal(err)
	}
	scrivi("file leggibile da tutti", "seconda")

	bersaglio, err := filepath.Abs("bersaglio")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bersaglio, []byte("intatto"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(fileAdminPassword); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bersaglio, fileAdminPassword); err != nil {
		t.Fatal(err)
	}
	scrivi("link simbolico", "terza")
	if data, err := os.ReadFile(bersaglio); err != nil || string(data) != "intatto" {
		t.Errorf("il bersaglio del link contiene %q (%v), atteso \"intatto\"", data, err)
	}
}
