package service

import (
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// TestComandoServer prova i comandi al server rustdesk con un server finto
// che legge la richiesta e risponde "ok", o non risponde mai. Senza risposta
// SendSocketCmd rinuncia dopo tempoComandoServer col tempo scaduto: prima la
// lettura della risposta aspettava senza limite, e con lei la richiesta del
// pannello.
func TestComandoServer(t *testing.T) {
	for _, tc := range []struct {
		nome     string
		risponde bool
	}{
		{"risponde", true},
		{"non risponde", false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			registroDiProva(t)
			precTempo := tempoComandoServer
			tempoComandoServer = 200 * time.Millisecond
			t.Cleanup(func() { tempoComandoServer = precTempo })
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ln.Close() })
			go func() {
				for {
					c, err := ln.Accept()
					if err != nil {
						return
					}
					go func() {
						if tc.risponde {
							_, _ = c.Read(make([]byte, 64))
							_, _ = io.WriteString(c, "ok")
						} else {
							_, _ = io.Copy(io.Discard, c)
						}
						_ = c.Close()
					}()
				}
			}()

			type esito struct {
				risposta string
				err      error
			}
			fatto := make(chan esito, 1)
			go func() {
				risposta, err := (&ServerCmdService{}).SendSocketCmd("v4", ln.Addr().(*net.TCPAddr).Port, "h")
				fatto <- esito{risposta, err}
			}()
			select {
			case e := <-fatto:
				if tc.risponde && (e.err != nil || e.risposta != "ok") {
					t.Errorf("SendSocketCmd: risposta %q, errore %v; attesi ok e nessun errore", e.risposta, e.err)
				}
				if !tc.risponde && !errors.Is(e.err, os.ErrDeadlineExceeded) {
					t.Errorf("SendSocketCmd: errore %v, atteso il tempo scaduto", e.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("SendSocketCmd fermo da 5 secondi")
			}
		})
	}
}
