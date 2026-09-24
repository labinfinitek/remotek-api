package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

var formatoToken = regexp.MustCompile(`^[0-9a-f]{32}$`)

// senzaChiaveJwt fa generare a GenerateToken il token casuale, non il JWT.
func senzaChiaveJwt(t *testing.T) {
	t.Helper()
	prec := Jwt
	Jwt = &jwt.Jwt{}
	t.Cleanup(func() { Jwt = prec })
}

func TestGenerateTokenFormato(t *testing.T) {
	senzaChiaveJwt(t)
	token := (&UserService{}).GenerateToken(&model.User{Username: "collaudo"})
	if !formatoToken.MatchString(token) {
		t.Fatalf("token %q: servono 32 caratteri esadecimali minuscoli", token)
	}
}

func TestGenerateTokenDiversiPerLoStessoUtente(t *testing.T) {
	senzaChiaveJwt(t)
	us := &UserService{}
	u := &model.User{Username: "collaudo"}
	visti := map[string]bool{}
	for range 1000 {
		token := us.GenerateToken(u)
		if visti[token] {
			t.Fatalf("token %q generato due volte per lo stesso utente", token)
		}
		visti[token] = true
	}
}

// TestGenerateTokenNonDipendeDaUtenteEOra fa quello che farebbe un
// attaccante col vecchio codice: conosce il nome utente e l'intervallo in cui
// e' stato fatto il login, e prova md5(nome utente + ora) per ogni istante
// dell'intervallo.
func TestGenerateTokenNonDipendeDaUtenteEOra(t *testing.T) {
	senzaChiaveJwt(t)
	us := &UserService{}
	u := &model.User{Username: "collaudo"}
	us.GenerateToken(u) // la prima chiamata inizializza crypto/rand: fuori dall'intervallo
	// Serve un intervallo stretto, perche' i candidati crescono con la sua
	// durata: di solito sono 1-3 us anche con -race.
	for range 100 {
		prima := time.Now()
		token := us.GenerateToken(u)
		dopo := time.Now()
		if dopo.Sub(prima) > 20*time.Microsecond {
			continue
		}
		if ora, ok := oraDelToken(t, u.Username, token, prima, dopo); ok {
			t.Fatalf("il token %q e' md5(%q + %q): si indovina da nome utente e ora", token, u.Username, ora)
		}
		return
	}
	t.Fatal("in 100 tentativi GenerateToken non ha mai impiegato meno di 20 us")
}

// oraDelToken cerca tra prima e dopo l'ora che, formattata da time.String()
// e preceduta dal nome utente, ha come md5 il token. Lettura di sistema e
// lettura monotona di time.Now() non avanzano insieme al nanosecondo: si
// lasciano scostare di al piu' 256 ns.
func oraDelToken(t *testing.T, username, token string, prima, dopo time.Time) (string, bool) {
	t.Helper()
	const scarto = 256
	cercato, err := hex.DecodeString(token)
	if err != nil {
		return "", false
	}
	pw, dw := prima.UnixNano(), dopo.UnixNano()
	pm, dm := letturaMonotona(t, prima), letturaMonotona(t, dopo)
	monotone := make([]string, dm-pm+1)
	for i := range monotone {
		monotone[i] = formatoMonotono(pm + int64(i))
	}
	// La ricostruzione deve ridare time.String(), altrimenti non trovare
	// niente non dimostra niente.
	if got := time.Unix(0, pw).String() + monotone[0]; got != prima.String() {
		t.Fatalf("ricostruzione dell'ora: %q, time.String() da' %q", got, prima.String())
	}
	var buf []byte
	for w := pw; w <= dw; w++ {
		muro := username + time.Unix(0, w).String()
		for m := max(pm, pm+(w-pw)-scarto); m <= min(dm, pm+(w-pw)+scarto); m++ {
			buf = append(append(buf[:0], muro...), monotone[m-pm]...)
			if sum := md5.Sum(buf); bytes.Equal(sum[:], cercato) {
				return strings.TrimPrefix(string(buf), username), true
			}
		}
	}
	return "", false
}

// letturaMonotona restituisce in nanosecondi la parte " m=±s.nnnnnnnnn" che
// time.String() aggiunge a un'ora letta con time.Now().
func letturaMonotona(t *testing.T, ora time.Time) int64 {
	t.Helper()
	s := ora.String()
	i := strings.LastIndex(s, " m=")
	if i < 0 {
		t.Fatalf("%q: manca la lettura monotona", s)
	}
	sec, ns, ok := strings.Cut(s[i+len(" m="):], ".")
	neg := strings.HasPrefix(sec, "-")
	a, errA := strconv.ParseInt(strings.TrimLeft(sec, "+-"), 10, 64)
	b, errB := strconv.ParseInt(ns, 10, 64)
	if !ok || errA != nil || errB != nil {
		t.Fatalf("%q: lettura monotona illeggibile", s)
	}
	if neg {
		return -(a*1e9 + b)
	}
	return a*1e9 + b
}

func formatoMonotono(m int64) string {
	segno := '+'
	if m < 0 {
		segno, m = '-', -m
	}
	return fmt.Sprintf(" m=%c%d.%09d", segno, m/1e9, m%1e9)
}

func TestTokenCasualeUsaSoloIByteLetti(t *testing.T) {
	letti := []byte("0123456789abcdef")
	got, err := tokenCasuale(bytes.NewReader(letti))
	if err != nil {
		t.Fatalf("tokenCasuale: %v", err)
	}
	if want := hex.EncodeToString(letti); got != want {
		t.Fatalf("token %q, atteso %q: il token deve essere solo i 16 byte casuali", got, want)
	}
}

func TestTokenCasualeSenzaCasoNessunToken(t *testing.T) {
	guasto := errors.New("fonte casuale guasta")
	casi := map[string]struct {
		lettore io.Reader
		err     error
	}{
		"errore":            {iotest.ErrReader(guasto), guasto},
		"byte troppo pochi": {bytes.NewReader([]byte("corto")), nil},
	}
	for nome, c := range casi {
		t.Run(nome, func(t *testing.T) {
			token, err := tokenCasuale(c.lettore)
			if err == nil || token != "" {
				t.Fatalf("token %q, errore %v: serve un errore e nessun token", token, err)
			}
			if c.err != nil && !errors.Is(err, c.err) {
				t.Fatalf("errore %v: non avvolge %v", err, c.err)
			}
		})
	}
}
