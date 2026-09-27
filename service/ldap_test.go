package service

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/lejianwen/rustdesk-api/v2/config"
)

// registroDiProva fa scrivere il log dei servizi in un registro, che
// restituisce.
func registroDiProva(t *testing.T) *strings.Builder {
	t.Helper()
	registro := &strings.Builder{}
	prec := Logger
	Logger = logrus.New()
	Logger.SetOutput(registro)
	t.Cleanup(func() { Logger = prec })
	return registro
}

// serverLdapCheRifiuta avvia un server LDAP finto che a ogni bind risponde
// invalidCredentials (49) e ne restituisce l'indirizzo ldap://.
func serverLdapCheRifiuta(t *testing.T) string {
	t.Helper()
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
			go rispondiAlBind(c)
		}
	}()
	return "ldap://" + ln.Addr().String()
}

// rispondiAlBind legge da c una richiesta LDAP, risponde al suo messageID
// con un BindResponse invalidCredentials e aspetta che il client chiuda.
func rispondiAlBind(c net.Conn) {
	defer func() { _ = c.Close() }()
	// La richiesta e' una SEQUENCE (0x30) con la lunghezza in forma breve o
	// lunga; il suo primo campo e' il messageID, un INTEGER (0x02).
	testa := make([]byte, 2)
	if _, err := io.ReadFull(c, testa); err != nil || testa[0] != 0x30 {
		return
	}
	n := int(testa[1])
	if n&0x80 != 0 {
		lunghezza := make([]byte, n&0x7f)
		if _, err := io.ReadFull(c, lunghezza); err != nil {
			return
		}
		n = 0
		for _, b := range lunghezza {
			n = n<<8 | int(b)
		}
	}
	corpo := make([]byte, n)
	if _, err := io.ReadFull(c, corpo); err != nil || n < 2 || corpo[0] != 0x02 || n < 2+int(corpo[1]) {
		return
	}
	id := corpo[:2+int(corpo[1])]
	// BindResponse, [APPLICATION 1]: resultCode 49, matchedDN e
	// diagnosticMessage vuoti.
	risposta := append(append([]byte{}, id...), 0x61, 0x07, 0x0a, 0x01, 49, 0x04, 0x00, 0x04, 0x00)
	if _, err := c.Write(append([]byte{0x30, byte(len(risposta))}, risposta...)); err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, c)
}

// TestBindLdapRifiutato prova che un bind LDAP rifiutato va nel log dei
// servizi col motivo e che connectAndBind restituisce ErrLdapBindService:
// prima "Bind failed" si stampava su stdout, fuori dal log e senza motivo.
func TestBindLdapRifiutato(t *testing.T) {
	registro := registroDiProva(t)
	cfg := &config.Ldap{Url: serverLdapCheRifiuta(t)}
	conn, err := (&LdapService{}).connectAndBind(cfg, "cn=servizio,dc=esempio,dc=it", "password-sbagliata")
	if conn != nil || !errors.Is(err, ErrLdapBindService) {
		t.Fatalf("connectAndBind: connessione %v, errore %v; attesi nessuna connessione e ErrLdapBindService", conn, err)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "level=warning") || !strings.Contains(nelLog, "Invalid Credentials") {
		t.Errorf("nel log manca il bind rifiutato col motivo:\n%s", nelLog)
	}
}

// TestUserAccountControlNonNumerico prova che un userAccountControl che non
// e' un numero rende l'utente disabilitato e va nel log dei servizi a
// livello error, col nome dell'utente e il valore: prima si stampava su
// stdout.
func TestUserAccountControlNonNumerico(t *testing.T) {
	registro := registroDiProva(t)
	cfg := &config.Ldap{User: config.LdapUser{EnableAttr: "userAccountControl", EnableAttrValue: "512"}}
	lu := &LdapUser{Username: "mrossi", EnableAttrValue: "non-numero"}
	if (&LdapService{}).isUserEnabled(cfg, lu) || lu.Enabled {
		t.Errorf("utente con userAccountControl %q: abilitato, atteso disabilitato", lu.EnableAttrValue)
	}
	if nelLog := registro.String(); !strings.Contains(nelLog, "level=error") || !strings.Contains(nelLog, "mrossi") || !strings.Contains(nelLog, "non-numero") {
		t.Errorf("nel log manca l'errore di userAccountControl:\n%s", nelLog)
	}
}
