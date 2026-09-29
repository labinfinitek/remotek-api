package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestArgomentiSbagliati prova sul binario vero che un errore di cobra che
// arriva prima di InitGlobal, come gli argomenti sbagliati di un sottocomando,
// esca con codice 1 e il messaggio di cobra su stderr, senza panic: prima
// main scriveva nel logger, che non c'era ancora, e il processo usciva con 2.
func TestArgomentiSbagliati(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"reset-admin-pwd"},
		{"reset-pwd", "1"},
		{"inesistente"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), bin, args...)
			cmd.Dir = sandbox(t)
			cmd.Env = append(os.Environ(), figlio+"=1", "PWD="+cmd.Dir)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var uscita *exec.ExitError
			if err != nil && !errors.As(err, &uscita) {
				t.Fatal(err)
			}
			if codice := cmd.ProcessState.ExitCode(); codice != 1 {
				t.Errorf("codice %d, atteso 1\nstdout:\n%s\nstderr:\n%s", codice, &stdout, &stderr)
			}
			if !strings.HasPrefix(stderr.String(), "Error: ") {
				t.Errorf("stderr senza il messaggio di cobra:\n%s", &stderr)
			}
			if tutto := stdout.String() + stderr.String(); strings.Contains(tutto, "panic") {
				t.Errorf("panic nell'uscita:\n%s", tutto)
			}
		})
	}
}
