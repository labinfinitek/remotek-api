package service

import (
	"regexp"
	"testing"
)

// TestCodiceAssociazione prova che il codice abbia 8 caratteri leggibili al
// telefono, senza 0/O e 1/I/L, a gruppi di 4, e che l'amministratore lo
// possa scrivere minuscolo, con uno spazio o senza trattino.
func TestCodiceAssociazione(t *testing.T) {
	forma := regexp.MustCompile(`^[A-HJKMNP-Z2-9]{4}-[A-HJKMNP-Z2-9]{4}$`)
	for range 200 {
		c, err := nuovoCodice()
		if err != nil {
			t.Fatal(err)
		}
		if !forma.MatchString(c) {
			t.Fatalf("codice %q", c)
		}
	}
	for _, tc := range []struct{ scritto, atteso string }{
		{"k7q2-m9xd", "K7Q2-M9XD"}, {" K7Q2 M9XD ", "K7Q2-M9XD"}, {"K7Q2M9XD", "K7Q2-M9XD"}, {"K7Q", "K7Q"},
	} {
		if c := NormalizzaCodice(tc.scritto); c != tc.atteso {
			t.Errorf("NormalizzaCodice(%q) = %q, atteso %q", tc.scritto, c, tc.atteso)
		}
	}
}
