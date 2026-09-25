package main

import (
	"errors"
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

// TestLangNonSupportato prova sul binario vero che un lang che non e' una
// lingua dell'API, per esempio "zh-CN" di un file vecchio di upstream, fermi
// l'avvio con codice 1 e un messaggio che elenca i valori ammessi, prima di
// creare data/rustdeskapi.db; prima ripiegava sull'inglese senza dirlo. Il
// valore si confronta alla lettera, come gorm.type: "it-IT" e "IT" no. lang
// en e lang vuoto (l'inglese) partono.
func TestLangNonSupportato(t *testing.T) {
	for _, lang := range []string{"zh-CN", "es", "it-IT", "IT"} {
		t.Run(lang, func(t *testing.T) {
			dir := sandbox(t)
			t.Setenv("RUSTDESK_API_LANG", lang)
			codice, out := esegui(t, dir)
			messaggio := `lang "` + lang + `" non supportato, l'API non parte: i valori ammessi sono "en", "it", o vuoto per l'inglese`
			if codice != 1 || !strings.Contains(out, messaggio) {
				t.Errorf("codice %d, attesi 1 e %q nel messaggio\n%s", codice, messaggio, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "data", "rustdeskapi.db")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("data/rustdeskapi.db creato o illeggibile (%v): l'avvio doveva fermarsi prima", err)
			}
		})
	}

	t.Run("en", func(t *testing.T) {
		t.Setenv("RUSTDESK_API_LANG", "en")
		if codice, out := esegui(t, sandbox(t)); codice != 0 {
			t.Errorf("codice %d, atteso 0\n%s", codice, out)
		}
	})

	// lang vuoto: una variabile vuota per viper non c'e', quindi serve un
	// config.yaml che lo dica.
	t.Run("vuoto", func(t *testing.T) {
		dir := sandbox(t)
		conf, err := os.ReadFile(filepath.Join(dir, "conf", "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		vuoto := strings.Replace(string(conf), `lang: "it"`, `lang: ""`, 1)
		if vuoto == string(conf) {
			t.Fatal(`conf/config.yaml non ha lang: "it"`)
		}
		err = errors.Join(os.Remove(filepath.Join(dir, "conf")), os.Mkdir(filepath.Join(dir, "conf"), 0o750),
			os.WriteFile(filepath.Join(dir, "conf", "config.yaml"), []byte(vuoto), 0o600))
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("RUSTDESK_API_LANG", "")
		if codice, out := esegui(t, dir); codice != 0 {
			t.Errorf("codice %d, atteso 0\n%s", codice, out)
		}
	})
}
