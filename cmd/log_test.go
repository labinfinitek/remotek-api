package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// TestLivelloDiLogNonValido prova sul binario vero che un logger.level non
// valido, anche uno che logrus accettava, fermi l'avvio con codice 1 e una
// riga error che nomina i valori ammessi, prima di aprire il database.
func TestLivelloDiLogNonValido(t *testing.T) {
	for _, livello := range []string{"trace", "verbose"} {
		t.Run(livello, func(t *testing.T) {
			dir := sandbox(t)
			t.Setenv("RUSTDESK_API_LOGGER_LEVEL", livello)
			codice, out := esegui(t, dir)
			if codice != 1 {
				t.Errorf("codice %d, atteso 1\n%s", codice, out)
			}
			if logger.Conta(out, "ERROR", `logger.level "`+livello+`" non valido`, "debug, info, warn, error", "l'API non parte") != 1 {
				t.Errorf("manca la riga ERROR sul livello non valido\n%s", out)
			}
			if _, err := os.Stat(filepath.Join(dir, "data", "rustdeskapi.db")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("data/rustdeskapi.db creato o illeggibile (%v): l'avvio doveva fermarsi prima", err)
			}
		})
	}
}

// TestPercorsoDiLogIgnorato prova sul binario vero che con logger.path
// impostata l'avvio riesca, il file non nasca e un warn dica che la chiave
// e' ignorata; senza, nessun warn.
func TestPercorsoDiLogIgnorato(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 || logger.Conta(out, "WARN", "logger.path") != 0 {
		t.Fatalf("avvio senza logger.path: codice %d, o warn su logger.path\n%s", codice, out)
	}
	file := filepath.Join(dir, "runtime", "log.txt")
	t.Setenv("RUSTDESK_API_LOGGER_PATH", file)
	codice, out := esegui(t, dir)
	if codice != 0 {
		t.Fatalf("avvio: codice %d\n%s", codice, out)
	}
	if logger.Conta(out, "WARN", "logger.path", "ignorata") != 1 {
		t.Errorf("manca il warn su logger.path ignorata\n%s", out)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s creato o illeggibile (%v): il log va solo su stdout", file, err)
	}
}
