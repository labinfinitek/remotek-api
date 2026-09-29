package main

import (
	"strings"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/lib/logger"
)

// TestAvvisiLdap prova sul binario vero che l'avvio con LDAP acceso scriva
// un warn se ldap.url e' ldap:// (password in chiaro) e uno se ldaps:// ha
// ldap.tls-verify false (certificati non verificati), senza l'url, che puo'
// contenere utente e password; nessun warn con ldaps:// e il default, ne'
// con LDAP spento.
func TestAvvisiLdap(t *testing.T) {
	const (
		inChiaro      = "LDAP: ldap.url usa ldap:// senza TLS, le password del bind e degli utenti passano in chiaro"
		nonVerificato = "LDAP: ldap.tls-verify e' false, con ldaps:// i certificati del server non si verificano"
	)
	for _, tc := range []struct {
		caso, enable, url, verify, want string
	}{
		{"ldap://", "true", "ldap://servizio:segreto@ldap.example.com:389", "", inChiaro},
		{"ldaps:// con tls-verify false", "true", "ldaps://servizio:segreto@ldap.example.com:636", "false", nonVerificato},
		{"ldaps:// col default", "true", "ldaps://servizio:segreto@ldap.example.com:636", "", ""},
		{"LDAP spento", "false", "ldap://servizio:segreto@ldap.example.com:389", "false", ""},
	} {
		t.Run(tc.caso, func(t *testing.T) {
			t.Setenv("RUSTDESK_API_LDAP_ENABLE", tc.enable)
			t.Setenv("RUSTDESK_API_LDAP_URL", tc.url)
			t.Setenv("RUSTDESK_API_LDAP_TLS_VERIFY", tc.verify) // vuota: per viper non c'e'
			codice, out := esegui(t, sandbox(t))
			if codice != 0 {
				t.Fatalf("avvio: codice %d\n%s", codice, out)
			}
			for _, riga := range []string{inChiaro, nonVerificato} {
				n, want := logger.Conta(out, "WARN", riga), 0
				if riga == tc.want {
					want = 1
				}
				if n != want {
					t.Errorf("%d righe WARN %q, attese %d\n%s", n, riga, want, out)
				}
			}
			if strings.Contains(out, "segreto") {
				t.Errorf("la password dell'url e' nel log\n%s", out)
			}
		})
	}
}
