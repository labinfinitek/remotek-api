package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	applog "github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// TestAvvioIntegrazione prova sul binario vero che con integrazione.url
// impostato l'avvio si fermi, con codice 1 e una riga ERROR, prima di
// creare il database, se l'URL non e' https://, ha credenziali o il
// segreto ha meno di 32 caratteri; il log non ha ne' l'URL ne' il
// segreto. Con URL e segreto giusti l'avvio va avanti.
func TestAvvioIntegrazione(t *testing.T) {
	const segreto = "segreto-di-prova-di-almeno-32-caratteri"
	for _, tc := range []struct{ caso, url, segreto, messaggio string }{
		{"http", "http://n8n.example/webhook/abc", segreto, "integrazione.url deve iniziare con https://"},
		{"credenziali", "https://utente:parola@n8n.example/webhook/abc", segreto, "integrazione.url non deve contenere credenziali"},
		{"segreto corto", "https://n8n.example/webhook/abc", "corto-corto-corto", "integrazione.segreto deve avere almeno 32 caratteri"},
		{"segreto vuoto", "https://n8n.example/webhook/abc", "", "integrazione.segreto deve avere almeno 32 caratteri"},
		{"giusta", "https://n8n.example/webhook/abc", segreto, ""},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			dir := sandbox(t)
			t.Setenv("RUSTDESK_API_INTEGRAZIONE_URL", tc.url)
			t.Setenv("RUSTDESK_API_INTEGRAZIONE_SEGRETO", tc.segreto)
			codice, out := esegui(t, dir)
			_, errDB := os.Stat(filepath.Join(dir, "data", "rustdeskapi.db"))
			if tc.messaggio == "" {
				if codice != 0 || errDB != nil {
					t.Fatalf("codice %d (atteso 0), database: %v\n%s", codice, errDB, out)
				}
			} else {
				if codice != 1 || applog.Conta(out, "ERROR", tc.messaggio+", l'API non parte") != 1 {
					t.Errorf("codice %d (atteso 1), riga ERROR %q:\n%s", codice, tc.messaggio, out)
				}
				if !errors.Is(errDB, os.ErrNotExist) {
					t.Errorf("data/rustdeskapi.db creato o illeggibile (%v): l'avvio doveva fermarsi prima", errDB)
				}
			}
			for _, s := range []string{"webhook/abc", "parola", segreto, "corto-corto"} {
				if strings.Contains(out, s) {
					t.Errorf("nel log c'e' %q:\n%s", s, out)
				}
			}
		})
	}
}

// TestComandoTokenIntegrazione prova sul binario vero token-integrazione:
// crea stampa il token una volta sola, e nel database c'e' solo il suo
// hash; elenco non mostra ne' token ne' hash; revoca segna la revoca; un
// nome ripetuto o un token che non c'e' escono con 1.
func TestComandoTokenIntegrazione(t *testing.T) {
	dir := sandbox(t)
	codice, out := esegui(t, dir, "token-integrazione", "crea", "n8n")
	token := regexp.MustCompile(`(?m)^[A-Za-z0-9_-]{43}$`).FindString(out)
	if codice != 0 || token == "" || strings.Count(out, token) != 1 {
		t.Fatalf("crea: codice %d, token %q\n%s", codice, token, out)
	}
	var hash string
	if err := apriDB(t, dir).Raw("SELECT hash FROM token_integrazione WHERE nome = 'n8n'").Scan(&hash).Error; err != nil || len(hash) != 64 {
		t.Fatalf("hash %q (%v)", hash, err)
	}
	if codice, out := esegui(t, dir, "token-integrazione", "crea", "n8n"); codice != 1 || strings.Contains(out, token) {
		t.Errorf("crea ripetuto: codice %d, atteso 1\n%s", codice, out)
	}
	elenco := func() string {
		t.Helper()
		codice, out := esegui(t, dir, "token-integrazione", "elenco")
		if codice != 0 || !strings.Contains(out, "n8n\t") || strings.Contains(out, token) || strings.Contains(out, hash) {
			t.Fatalf("elenco: codice %d\n%s", codice, out)
		}
		return out
	}
	if out := elenco(); !regexp.MustCompile(`n8n\t\S+Z\t\S+Z\t-\t-`).MatchString(out) {
		t.Errorf("elenco prima della revoca:\n%s", out)
	}
	if codice, out := esegui(t, dir, "token-integrazione", "revoca", "n8n"); codice != 0 {
		t.Fatalf("revoca: codice %d\n%s", codice, out)
	}
	if out := elenco(); !regexp.MustCompile(`n8n\t\S+Z\t\S+Z\t\S+Z\t-`).MatchString(out) {
		t.Errorf("elenco dopo la revoca:\n%s", out)
	}
	if codice, out := esegui(t, dir, "token-integrazione", "revoca", "manca"); codice != 1 {
		t.Errorf("revoca di un token che non c'e': codice %d, atteso 1\n%s", codice, out)
	}
}
