package config

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// Agente e' la configurazione degli agenti AI (ADR-0028).
// LasciapassareChiave e' il percorso del file col seme Ed25519 con cui si
// firmano i lasciapassare: vuoto, la funzione e' spenta. Il seme non va mai
// nel log.
type Agente struct {
	LasciapassareChiave string `mapstructure:"lasciapassare-chiave"`
	chiave              ed25519.PrivateKey
}

// Carica legge la chiave da LasciapassareChiave, se e' impostato. Il file
// deve esistere, non essere leggibile ne' scrivibile da gruppo e altri e
// avere su una riga il seme di 32 byte in base64; altrimenti un errore che
// dice cosa non va, mai col contenuto del file.
func (a *Agente) Carica() error {
	a.chiave = nil
	if a.LasciapassareChiave == "" {
		return nil
	}
	nome := "agente.lasciapassare-chiave " + a.LasciapassareChiave
	st, err := os.Stat(a.LasciapassareChiave)
	if err != nil {
		return fmt.Errorf("%s: file non letto: %w", nome, err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%s: non e' un file", nome)
	}
	if p := st.Mode().Perm(); p&0o077 != 0 {
		return fmt.Errorf("%s: permessi %o, servono 600 o piu' stretti (chmod 600)", nome, p)
	}
	b, err := os.ReadFile(a.LasciapassareChiave)
	if err != nil {
		return fmt.Errorf("%s: file non letto: %w", nome, err)
	}
	seme, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(b)))
	if err != nil || len(seme) != ed25519.SeedSize {
		return fmt.Errorf("%s: non contiene un seme Ed25519 di 32 byte in base64 su una riga", nome)
	}
	a.chiave = ed25519.NewKeyFromSeed(seme)
	return nil
}

// Chiave e' la chiave privata caricata da Carica, nil se la funzione e'
// spenta.
func (a *Agente) Chiave() ed25519.PrivateKey { return a.chiave }

// Pubblica e' la chiave pubblica in base64, vuota se la funzione e' spenta:
// non e' un segreto, e serve a confrontarla con quella cablata nel client.
func (a *Agente) Pubblica() string {
	if a.chiave == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(a.chiave.Public().(ed25519.PublicKey))
}

// ChiaveNuova genera con crypto/rand una coppia Ed25519, scrive il seme in
// base64 nel file percorso, che non deve esistere, con permessi 600, e
// restituisce la chiave pubblica in base64.
func ChiaveNuova(percorso string) (string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("chiave casuale: %w", err)
	}
	f, err := os.OpenFile(percorso, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%s esiste gia': non si sovrascrive una chiave", percorso)
	}
	if err != nil {
		return "", fmt.Errorf("file della chiave: %w", err)
	}
	// Chmod porta a 600 esatti anche con una umask insolita.
	if err = f.Chmod(0o600); err == nil {
		if _, err = f.WriteString(base64.StdEncoding.EncodeToString(priv.Seed()) + "\n"); err == nil {
			err = f.Sync()
		}
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return "", fmt.Errorf("file della chiave %s: %w", percorso, err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}
