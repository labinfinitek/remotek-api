package service

import (
	"regexp"
	"runtime"
	"strconv"
	"testing"
	"time"
	"weak"

	"github.com/lejianwen/rustdesk-api/v2/model"
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

// TestRichiesteNonRestano prova che la pulizia a tempo toglie una richiesta
// scaduta, e con lei nome ed email, anche se nessuno legge le richieste, e
// che la richiesta espulsa dal tetto non e' piu' raggiungibile: il garbage
// collector la libera. Prima la scaduta restava fino alla lettura dopo, e
// l'espulsa restava nell'array sotto la fetta.
func TestRichiesteNonRestano(t *testing.T) {
	inMemoria := func(r *Richieste) (n int) {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, v := range r.voci[:cap(r.voci)] {
			if v != nil {
				n++
			}
		}
		return n
	}

	// Nuova legge l'ora due volte (pulizia e creazione); dalla terza, quella
	// della pulizia a tempo, la richiesta e' scaduta. Spostare l'orologio dal
	// test dopo Nuova era una corsa col timer: se scattava prima, la pulizia
	// non toglieva niente e nessun'altra la ripeteva.
	adesso, letture := time.Now(), 0
	ora := func() time.Time {
		if letture++; letture > 2 {
			return adesso.Add(DurataRichiesta)
		}
		return adesso
	}
	r := &Richieste{Ora: ora, attesa: 10 * time.Millisecond}
	if _, err := r.Nuova("aziendale", &model.OauthUser{OpenId: "sub-1", Name: "Utente", Email: "utente@esempio.it"}); err != nil {
		t.Fatal(err)
	}
	for scadenza := time.Now().Add(5 * time.Second); inMemoria(r) > 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(scadenza) {
			t.Fatal("la richiesta scaduta e' ancora in memoria dopo la pulizia")
		}
	}

	r = &Richieste{Ora: time.Now}
	var prima weak.Pointer[RichiestaAssociazione]
	for i := range TettoRichieste + 1 {
		if i == TettoRichieste {
			// la piu' vecchia, che la prossima fa uscire, senza tenerla viva
			r.mu.Lock()
			prima = weak.Make(r.voci[0])
			r.mu.Unlock()
		}
		if _, err := r.Nuova("aziendale", &model.OauthUser{OpenId: "sub-" + strconv.Itoa(i), Email: "utente@esempio.it"}); err != nil {
			t.Fatal(err)
		}
	}
	runtime.GC()
	if v := prima.Value(); v != nil {
		t.Errorf("la richiesta espulsa dal tetto (%s) e' ancora in memoria", v.Sub)
	}
	if n := inMemoria(r); n != TettoRichieste {
		t.Errorf("richieste nell'array: %d, attese %d", n, TettoRichieste)
	}
}
