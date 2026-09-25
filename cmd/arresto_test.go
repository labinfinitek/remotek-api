//go:build !windows

package main

// Test dell'avvio e dello stop del server HTTP sul binario vero, in un
// processo figlio: il codice d'uscita e' quello che vede systemd o Docker.
// Solo fuori da Windows, come endless (http/run.go).

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// avviaServer fa partire in dir il figlio col server API su addr e
// restituisce il processo avviato e il buffer che raccoglie cio' che scrive.
func avviaServer(t *testing.T, dir, addr string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)
	// Un argomento qualsiasi fa girare rootCmd col server, non il solo
	// bootstrap (TestMain): -c col valore predefinito.
	cmd := exec.CommandContext(ctx, bin, "-c", "./conf/config.yaml")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), figlio+"=1", "PWD="+dir, "RUSTDESK_API_GIN_API_ADDR="+addr)
	out := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, out
}

// codice aspetta la fine del figlio e ne restituisce il codice d'uscita.
func codice(t *testing.T, cmd *exec.Cmd, out *bytes.Buffer) int {
	t.Helper()
	err := cmd.Wait()
	var uscita *exec.ExitError
	if err != nil && !errors.As(err, &uscita) {
		t.Fatalf("figlio: %v\n%s", err, out)
	}
	return cmd.ProcessState.ExitCode()
}

// TestPortaOccupata avvia il server su una porta gia' in uso: deve fermarsi
// con codice 1 e scrivere l'errore nel log. Prima usciva con codice 0.
func TestPortaOccupata(t *testing.T) {
	dir := sandbox(t)
	occupata, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupata.Close()
	cmd, out := avviaServer(t, dir, occupata.Addr().String())
	if c := codice(t, cmd, out); c != 1 {
		t.Errorf("codice %d, atteso 1\n%s", c, out)
	}
	registro, err := os.ReadFile(filepath.Join(dir, "runtime", "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"server API fermato", occupata.Addr().String(), "address already in use"} {
		if !strings.Contains(string(registro), s) {
			t.Errorf("il log non contiene %q:\n%s", s, registro)
		}
	}
}

// TestStopConSIGTERM avvia il server, aspetta che accetti connessioni e gli
// manda SIGTERM: lo stop normale deve uscire con codice 0.
func TestStopConSIGTERM(t *testing.T) {
	dir := sandbox(t)
	libera, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := libera.Addr().String()
	if err := libera.Close(); err != nil {
		t.Fatal(err)
	}
	cmd, out := avviaServer(t, dir, addr)
	scadenza := time.Now().Add(time.Minute)
	for {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(scadenza) {
			_ = cmd.Process.Kill()
			t.Fatalf("il server non accetta connessioni su %s: %v\n%s", addr, err, out)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if c := codice(t, cmd, out); c != 0 {
		t.Errorf("codice %d dopo SIGTERM, atteso 0\n%s", c, out)
	}
}
