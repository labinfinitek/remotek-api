// Package oidcfinto firma gli id_token dei provider OIDC finti dei test, con
// una chiave RS256 generata al volo e pubblicata come JWKS.
package oidcfinto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
)

// Chiave e' la chiave di firma di un provider finto.
type Chiave struct {
	privata *rsa.PrivateKey
}

// kid e' l'identificativo della chiave, nell'intestazione e nel JWKS.
const kid = "chiave-di-prova"

// Nuova genera una chiave RSA a 2048 bit.
func Nuova(t testing.TB) *Chiave {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &Chiave{privata: k}
}

// JWKS restituisce il corpo del jwks_uri che pubblica la chiave.
func (c *Chiave) JWKS() (string, error) {
	b64 := base64.RawURLEncoding.EncodeToString
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid,
		"n": b64(c.privata.N.Bytes()),
		"e": b64(big.NewInt(int64(c.privata.E)).Bytes()),
	}}})
	if err != nil {
		return "", fmt.Errorf("JWKS: %w", err)
	}
	return string(jwks), nil
}

// IDToken restituisce il JWT firmato RS256 con i claims dati. Non riceve il
// testing.TB perche' lo chiama il gestore HTTP del provider finto, fuori
// dalla goroutine del test, dove t.Fatal non si puo' chiamare.
func (c *Chiave) IDToken(claims map[string]any) (string, error) {
	b64 := base64.RawURLEncoding.EncodeToString
	corpo, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("claims dell'id_token: %w", err)
	}
	firmato := b64([]byte(`{"alg":"RS256","typ":"JWT","kid":"`+kid+`"}`)) + "." + b64(corpo)
	h := sha256.Sum256([]byte(firmato))
	firma, err := rsa.SignPKCS1v15(rand.Reader, c.privata, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("firma dell'id_token: %w", err)
	}
	return firmato + "." + b64(firma), nil
}
