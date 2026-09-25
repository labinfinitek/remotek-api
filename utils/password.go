package utils

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// EncryptPassword hashes the input password using bcrypt.
// An error is returned if hashing fails.
func EncryptPassword(password string) (string, error) {
	bs, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// VerifyPassword dice se input e' la password salvata nell'hash bcrypt hash.
// Una password diversa da' false senza errore. Un hash che non e' bcrypt, come
// l'md5(password+"rustdesk-api") delle versioni molto vecchie di rustdesk-api,
// non vale per nessuna password: false con l'errore di bcrypt, e chi chiama lo
// tratta come una password sbagliata. Va reimpostato con reset-pwd.
func VerifyPassword(hash, input string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return err == nil, err
}
