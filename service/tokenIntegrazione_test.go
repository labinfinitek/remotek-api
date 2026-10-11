package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"github.com/lejianwen/rustdesk-api/v2/lib/orm"
	"github.com/lejianwen/rustdesk-api/v2/model"
)

// tokenDiProva prepara un database con la tabella dei token di
// integrazione.
func tokenDiProva(t *testing.T) *TokenIntegrazioneService {
	t.Helper()
	db, err := orm.ApriSqlite(filepath.Join(t.TempDir(), "api.db"), logger.Discard)
	if err == nil {
		err = db.AutoMigrate(&model.TokenIntegrazione{})
	}
	if err != nil {
		t.Fatal(err)
	}
	prec := DB
	t.Cleanup(func() { DB = prec })
	DB = db
	return &TokenIntegrazioneService{}
}

// TestTokenIntegrazione: il token creato ha 32 byte casuali in base64url,
// nel database c'e' solo il suo SHA-256, vale un anno e segna l'ultimo uso;
// scaduto, revocato o sconosciuto non vale; il nome non si ripete.
func TestTokenIntegrazione(t *testing.T) {
	ts := tokenDiProva(t)
	adesso := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	token, err := ts.Crea("n8n", adesso)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(b) != 32 {
		t.Fatalf("token %q: %d byte (%v), attesi 32 in base64url", token, len(b), err)
	}
	var righe []map[string]any
	if err := DB.Raw("SELECT * FROM token_integrazione").Scan(&righe).Error; err != nil || len(righe) != 1 {
		t.Fatalf("righe %v (%v)", righe, err)
	}
	h := sha256.Sum256([]byte(token))
	for k, v := range righe[0] {
		if s, ok := v.(string); ok && strings.Contains(s, token) {
			t.Errorf("colonna %s contiene il token", k)
		}
	}
	if righe[0]["hash"] != hex.EncodeToString(h[:]) {
		t.Errorf("hash %v, atteso lo SHA-256 del token", righe[0]["hash"])
	}
	if _, err := ts.Crea("n8n", adesso); !errors.Is(err, ErrNomeUsato) {
		t.Errorf("secondo token n8n: %v, atteso ErrNomeUsato", err)
	}
	for _, nome := range []string{"", "x", "con spazio", strings.Repeat("a", 65)} {
		if _, err := ts.Crea(nome, adesso); err == nil {
			t.Errorf("nome %q accettato", nome)
		}
	}

	uso := adesso.Add(time.Hour)
	tk, err := ts.Verifica(token, uso)
	if err != nil || tk.Nome != "n8n" {
		t.Fatalf("Verifica: %v, %v", tk, err)
	}
	elenco, err := ts.Elenco()
	if err != nil || len(elenco) != 1 || elenco[0].UltimoUso != uso.Unix() || elenco[0].Scade != adesso.Add(DurataTokenIntegrazione).Unix() {
		t.Fatalf("elenco %+v (%v)", elenco, err)
	}
	if _, err := ts.Verifica(token, adesso.Add(DurataTokenIntegrazione)); !errors.Is(err, ErrNotFound) {
		t.Errorf("token scaduto: %v, atteso ErrNotFound", err)
	}
	if _, err := ts.Verifica("sconosciuto", uso); !errors.Is(err, ErrNotFound) {
		t.Errorf("token sconosciuto: %v, atteso ErrNotFound", err)
	}
	if err := ts.Revoca("n8n", uso); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Verifica(token, uso); !errors.Is(err, ErrNotFound) {
		t.Errorf("token revocato: %v, atteso ErrNotFound", err)
	}
	if err := ts.Revoca("manca", uso); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoca di un token che non c'e': %v, atteso ErrNotFound", err)
	}
}
