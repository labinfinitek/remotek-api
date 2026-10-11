package main

import (
	"errors"
	"os"
	"path/filepath"
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
