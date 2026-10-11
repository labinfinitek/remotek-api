package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// DurataTokenIntegrazione e' quanto vale un token di integrazione.
const DurataTokenIntegrazione = 365 * 24 * time.Hour

// ErrNomeUsato dice che c'e' gia' un token di integrazione con quel nome.
var ErrNomeUsato = errors.New("nome del token di integrazione gia' usato")

// nomeToken sono i nomi ammessi: da 2 a 64 tra lettere, cifre, punto,
// trattino e trattino basso.
var nomeToken = regexp.MustCompile(`^[A-Za-z0-9._-]{2,64}$`)

// hashToken e' lo SHA-256 esadecimale del token, l'unica cosa che si salva.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type TokenIntegrazioneService struct {
}

// Crea crea il token di integrazione nome, che scade un anno dopo adesso,
// e restituisce il token: 32 byte casuali in base64url, che non si salva e
// non si puo' piu' rileggere.
func (ts *TokenIntegrazioneService) Crea(nome string, adesso time.Time) (string, error) {
	if !nomeToken.MatchString(nome) {
		return "", errors.New("nome del token di integrazione non valido: da 2 a 64 tra lettere, cifre, '.', '-' e '_'")
	}
	var n int64
	if err := DB.Model(&model.TokenIntegrazione{}).Where("nome = ?", nome).Count(&n).Error; err != nil {
		return "", fmt.Errorf("token di integrazione %s: %w", nome, err)
	}
	if n > 0 {
		return "", fmt.Errorf("%w: %s", ErrNomeUsato, nome)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token di integrazione casuale: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	t := &model.TokenIntegrazione{Nome: nome, Hash: hashToken(token), Creato: adesso.Unix(),
		Scade: adesso.Add(DurataTokenIntegrazione).Unix()}
	if err := DB.Create(t).Error; err != nil {
		return "", fmt.Errorf("token di integrazione %s: %w", nome, err)
	}
	return token, nil
}

// Elenco restituisce i token di integrazione in ordine di nome.
func (ts *TokenIntegrazioneService) Elenco() ([]model.TokenIntegrazione, error) {
	var res []model.TokenIntegrazione
	if err := DB.Order("nome").Find(&res).Error; err != nil {
		return nil, fmt.Errorf("token di integrazione: %w", err)
	}
	return res, nil
}

// Revoca revoca all'ora adesso il token di integrazione nome; ErrNotFound se
// non c'e'. Un token gia' revocato resta com'era.
func (ts *TokenIntegrazioneService) Revoca(nome string, adesso time.Time) error {
	t := &model.TokenIntegrazione{}
	if err := DB.Where("nome = ?", nome).First(t).Error; err != nil {
		return fmt.Errorf("token di integrazione %s: %w", nome, nonTrovato(err))
	}
	if t.Revocato != 0 {
		return nil
	}
	if err := DB.Model(t).Update("revocato", adesso.Unix()).Error; err != nil {
		return fmt.Errorf("revoca del token di integrazione %s: %w", nome, err)
	}
	return nil
}

// Verifica restituisce il token di integrazione token se c'e', non e'
// scaduto e non e' revocato, e ne segna l'ultimo uso all'ora adesso;
// ErrNotFound altrimenti, anche per un token scaduto o revocato, che si
// riconosce dal record restituito comunque.
func (ts *TokenIntegrazioneService) Verifica(token string, adesso time.Time) (*model.TokenIntegrazione, error) {
	t := &model.TokenIntegrazione{}
	if err := DB.Where("hash = ?", hashToken(token)).First(t).Error; err != nil {
		return nil, fmt.Errorf("token di integrazione: %w", nonTrovato(err))
	}
	if t.Revocato != 0 || adesso.Unix() >= t.Scade {
		return t, fmt.Errorf("token di integrazione %s scaduto o revocato: %w", t.Nome, ErrNotFound)
	}
	if err := DB.Model(t).Update("ultimo_uso", adesso.Unix()).Error; err != nil {
		return nil, fmt.Errorf("ultimo uso del token di integrazione %s: %w", t.Nome, err)
	}
	return t, nil
}
