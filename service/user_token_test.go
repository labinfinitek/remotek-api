package service

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"testing"
	"testing/iotest"

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

// TestGenerateTokenSoloByteCasuali verifica che il token sia fatto solo dei
// 16 byte letti dalla fonte casuale: niente nome utente, niente ora. Il
// vecchio md5(nome utente + ora) non lo passa.
func TestGenerateTokenSoloByteCasuali(t *testing.T) {
	senzaChiaveJwt(t)
	letti := []byte("0123456789abcdef")
	prec := fonteCasuale
	fonteCasuale = bytes.NewReader(letti)
	t.Cleanup(func() { fonteCasuale = prec })

	got := (&UserService{}).GenerateToken(&model.User{Username: "collaudo"})
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
