package utils

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// md5Vecchio e' l'hash delle versioni molto vecchie di rustdesk-api, che
// VerifyPassword accettava e riscriveva in bcrypt al login.
func md5Vecchio(password string) string {
	s := md5.Sum([]byte(password + "rustdesk-api"))
	return hex.EncodeToString(s[:])
}

// TestVerifyPasswordMD5: l'hash md5 non vale piu', neanche con la password
// giusta, e l'errore e' quello di bcrypt per un hash che non e' suo.
func TestVerifyPasswordMD5(t *testing.T) {
	ok, err := VerifyPassword(md5Vecchio("secret"), "secret")
	if ok {
		t.Fatal("un hash md5 e' stato accettato")
	}
	if !errors.Is(err, bcrypt.ErrHashTooShort) {
		t.Fatalf("errore %v, atteso %v", err, bcrypt.ErrHashTooShort)
	}
}

func TestVerifyPasswordBcrypt(t *testing.T) {
	b, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword(string(b), "pass"); err != nil || !ok {
		t.Fatalf("password giusta: ok %t, errore %v", ok, err)
	}
	if ok, err := VerifyPassword(string(b), "sbagliata"); err != nil || ok {
		t.Fatalf("password sbagliata: ok %t, errore %v; attesi false e nessun errore", ok, err)
	}
}

// TestVerifyPasswordMigrate: prima l'hash md5 si riscriveva in bcrypt; ora
// non si riscrive nulla e l'hash non vale per nessuna password, ne' quella
// da cui viene ne' l'hash stesso.
func TestVerifyPasswordMigrate(t *testing.T) {
	hash := md5Vecchio("mypass")
	for _, input := range []string{"mypass", hash, ""} {
		if ok, err := VerifyPassword(hash, input); ok || err == nil {
			t.Errorf("input %q: ok %t, errore %v; attesi false e l'errore di bcrypt", input, ok, err)
		}
	}
}
