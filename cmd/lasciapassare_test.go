package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// TestComandoChiaveNuova prova sul binario vero lasciapassare chiave-nuova:
// senza configurazione scrive il seme nel file con permessi 600, stampa
// su stdout solo la chiave pubblica, e rifiuta con 1 un file che esiste gia'.
// L'avvio con quel file scrive la stessa chiave pubblica in una riga INFO, e
// il seme non va mai nel log.
func TestComandoChiaveNuova(t *testing.T) {
	dir := sandbox(t)
	chiave := filepath.Join(dir, "data", "lasciapassare.key")
	codice, out := esegui(t, dir, "lasciapassare", "chiave-nuova", chiave)
	pub := regexp.MustCompile(`(?m)^[A-Za-z0-9+/]{43}=$`).FindString(out)
	if codice != 0 || pub == "" {
		t.Fatalf("chiave-nuova: codice %d, chiave pubblica %q\n%s", codice, pub, out)
	}
	st, err := os.Stat(chiave)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("file della chiave: %v, permessi %v", err, st.Mode().Perm())
	}
	b, _ := os.ReadFile(chiave)
	seme := strings.TrimSpace(string(b))
	if s, err := base64.StdEncoding.DecodeString(seme); err != nil || len(s) != 32 || strings.Contains(out, seme) {
		t.Fatalf("seme %d byte (%v), o stampato:\n%s", len(s), err, out)
	}
	if codice, out := esegui(t, dir, "lasciapassare", "chiave-nuova", chiave); codice != 1 || !strings.Contains(out, "esiste gia'") {
		t.Errorf("file esistente: codice %d, atteso 1\n%s", codice, out)
	}
	if dopo, _ := os.ReadFile(chiave); string(dopo) != string(b) {
		t.Error("il file esistente e' cambiato")
	}
	t.Setenv("RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE", chiave)
	codice, out = esegui(t, dir)
	if codice != 0 || applog.Conta(out, "INFO", "chiave pubblica "+pub) != 1 || strings.Contains(out, seme) {
		t.Errorf("avvio: codice %d, riga INFO con %s, seme nel log %t\n%s", codice, pub, strings.Contains(out, seme), out)
	}
}

// TestAvvioChiaveLasciapassare prova sul binario vero che con
// agente.lasciapassare-chiave impostato l'avvio si fermi, con codice 1 e una
// riga ERROR che dice cosa non va, prima di creare il database, se il file
// manca, e' leggibile da altri o non ha una chiave valida.
func TestAvvioChiaveLasciapassare(t *testing.T) {
	seme := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	for _, tc := range []struct {
		caso, testo string
		perm        os.FileMode
		messaggio   string
	}{
		{"file mancante", "", 0, "file non letto"},
		{"permessi 644", seme, 0o644, "permessi 644, servono 600 o piu' stretti"},
		{"chiave non valida", "corta", 0o600, "non contiene un seme Ed25519 di 32 byte"},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			dir := sandbox(t)
			chiave := filepath.Join(dir, "lasciapassare.key")
			if tc.perm != 0 {
				if err := os.WriteFile(chiave, []byte(tc.testo+"\n"), tc.perm); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(chiave, tc.perm); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("RUSTDESK_API_AGENTE_LASCIAPASSARE_CHIAVE", chiave)
			codice, out := esegui(t, dir)
			if codice != 1 || applog.Conta(out, "ERROR", tc.messaggio, "l'API non parte") != 1 {
				t.Errorf("codice %d (atteso 1), riga ERROR %q:\n%s", codice, tc.messaggio, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "data", "rustdeskapi.db")); !os.IsNotExist(err) {
				t.Errorf("data/rustdeskapi.db creato o illeggibile (%v): l'avvio doveva fermarsi prima", err)
			}
			if strings.Contains(out, seme) {
				t.Errorf("nel log c'e' il seme:\n%s", out)
			}
		})
	}
}
