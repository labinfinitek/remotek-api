package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileDiLinguaRotto avvia il binario vero con una cartella di lingue
// fatta apposta (gin.resources-path). Con en.toml valido e un file di un
// solo carattere, che non e' .toml, l'avvio riesce: il nome corto non manda
// in panic il controllo del suffisso. Con un it.toml rotto l'avvio si ferma
// con codice 1 e un messaggio che nomina il file: prima si saltava in
// silenzio e l'API rispondeva in inglese.
func TestFileDiLinguaRotto(t *testing.T) {
	dir := sandbox(t)
	lingue := filepath.Join(dir, "lingue", "i18n")
	if err := os.MkdirAll(lingue, 0o750); err != nil {
		t.Fatal(err)
	}
	scrivi := func(nome, contenuto string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(lingue, nome), []byte(contenuto), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	scrivi("en.toml", "[SystemError]\nother = \"System error.\"\n")
	scrivi("a", "")
	t.Setenv("RUSTDESK_API_GIN_RESOURCES_PATH", filepath.Dir(lingue))
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("file validi e un nome corto: codice %d, atteso 0\n%s", codice, out)
	}

	scrivi("it.toml", "[SystemError\nother = \"Errore di sistema.\"\n")
	if codice, out := esegui(t, dir); codice != 1 || !strings.Contains(out, "file di lingua it.toml non caricato") {
		t.Errorf("it.toml rotto: codice %d, attesi 1 e il nome del file nel messaggio\n%s", codice, out)
	}
}
