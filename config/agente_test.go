package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChiaveLasciapassare prova Carica: senza percorso la funzione e'
// spenta; un file che manca, leggibile da altri o senza un seme valido da'
// un errore che dice cosa non va, senza il contenuto del file; il file di
// ChiaveNuova si carica e da' la stessa chiave pubblica.
func TestChiaveLasciapassare(t *testing.T) {
	dir := t.TempDir()
	var spenta Agente
	if err := spenta.Carica(); err != nil || spenta.Chiave() != nil || spenta.Pubblica() != "" {
		t.Errorf("senza percorso: %v, chiave %v, pubblica %q", err, spenta.Chiave(), spenta.Pubblica())
	}
	scrivi := func(nome, testo string, perm os.FileMode) string {
		t.Helper()
		p := filepath.Join(dir, nome)
		if err := os.WriteFile(p, []byte(testo), perm); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, perm); err != nil {
			t.Fatal(err)
		}
		return p
	}
	seme := base64.StdEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct{ caso, path, messaggio string }{
		{"file che manca", filepath.Join(dir, "manca.key"), "file non letto"},
		{"permessi 644", scrivi("aperto.key", seme+"\n", 0o644), "permessi 644, servono 600 o piu' stretti"},
		{"permessi 640", scrivi("gruppo.key", seme+"\n", 0o640), "permessi 640"},
		{"non base64", scrivi("rotto.key", "non-e-base64!!\n", 0o600), "non contiene un seme Ed25519"},
		{"31 byte", scrivi("corto.key", base64.StdEncoding.EncodeToString(make([]byte, 31)), 0o600), "non contiene un seme Ed25519"},
		{"cartella", dir, "non e' un file"},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			a := Agente{LasciapassareChiave: tc.path}
			err := a.Carica()
			if err == nil || !strings.Contains(err.Error(), tc.messaggio) || !strings.HasPrefix(err.Error(), "agente.lasciapassare-chiave "+tc.path+": ") {
				t.Errorf("Carica: %v, atteso %q", err, tc.messaggio)
			}
			if err != nil && strings.Contains(err.Error(), "non-e-base64") {
				t.Errorf("nell'errore c'e' il contenuto del file: %v", err)
			}
			if a.Chiave() != nil {
				t.Error("chiave caricata nonostante l'errore")
			}
		})
	}
	p := scrivi("buona.key", seme+"\n", 0o400)
	a := Agente{LasciapassareChiave: p}
	if err := a.Carica(); err != nil {
		t.Fatalf("permessi 400: %v", err)
	}
	if want := base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)); a.Pubblica() != want {
		t.Errorf("pubblica %q, attesa %q", a.Pubblica(), want)
	}
}

// TestChiaveNuova prova ChiaveNuova: il file nasce con permessi 600 anche
// con una umask larga, ha il seme su una riga, si carica con la chiave
// pubblica restituita; un file che esiste gia' non si tocca.
func TestChiaveNuova(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lasciapassare.key")
	pub, err := ChiaveNuova(p)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("permessi %v (%v), attesi 600", st.Mode().Perm(), err)
	}
	b, _ := os.ReadFile(p)
	if strings.Count(string(b), "\n") != 1 {
		t.Errorf("il file non e' una riga: %q", b)
	}
	a := Agente{LasciapassareChiave: p}
	if err := a.Carica(); err != nil || a.Pubblica() != pub {
		t.Errorf("Carica: %v, pubblica %q, attesa %q", err, a.Pubblica(), pub)
	}
	if _, err := ChiaveNuova(p); err == nil || !strings.Contains(err.Error(), "esiste gia'") {
		t.Errorf("file esistente: %v", err)
	}
	if dopo, _ := os.ReadFile(p); string(dopo) != string(b) {
		t.Error("il file esistente e' cambiato")
	}
}
