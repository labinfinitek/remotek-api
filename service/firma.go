package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IntestazioneFirma porta la firma del dispositivo (ADR-0023): "<ts>.<firma>",
// con ts in secondi UNIX e firma la base64 standard dei 64 byte della firma
// Ed25519 di MessaggioFirma. Il formato e' nel README, "Firma del dispositivo".
const IntestazioneFirma = "X-Remotek-Firma"

// FinestraFirma e' lo scarto massimo tra il ts della firma e l'ora del
// server, ed e' anche quanto una firma accettata resta in memoria contro le
// ripetizioni.
const FinestraFirma = 300 * time.Second

// Motivi per cui una firma non basta; vanno nel log, mai firma ne' chiave.
const (
	FirmaAssente    = "firma assente"
	FirmaNonValida  = "firma non valida"
	FirmaFuoriTempo = "firma fuori tempo"
	FirmaRipetuta   = "firma ripetuta"
	SenzaChiave     = "firma di un PC senza chiave registrata"
)

// RichiestaFirmata e' cio' che serve a verificare la firma di una richiesta
// del client: metodo, percorso senza query, valore di IntestazioneFirma e
// corpo come e' arrivato. Pk e' il campo pk del sysinfo, vuoto altrove.
type RichiestaFirmata struct {
	Metodo, Percorso, Intestazione string
	Corpo                          []byte
	Pk                             string
}

// MessaggioFirma restituisce il messaggio che il dispositivo firma: cinque
// righe unite da "\n", senza "\n" finale.
func MessaggioFirma(metodo, percorso, ts string, corpo []byte) []byte {
	h := sha256.Sum256(corpo)
	return []byte(strings.Join([]string{"remotek-api-v1", strings.ToUpper(metodo), percorso, ts, hex.EncodeToString(h[:])}, "\n"))
}

// firmaValida dice se r e' firmata con la chiave pk (base64 di 32 byte),
// senza guardare l'ora ne' le ripetizioni; restituisce anche ts e firma.
func (r RichiestaFirmata) firmaValida(pk string) (bool, int64, []byte) {
	tsTesto, firmaB64, ok := strings.Cut(r.Intestazione, ".")
	if !ok || tsTesto == "" || len(tsTesto) > 19 || strings.Trim(tsTesto, "0123456789") != "" {
		return false, 0, nil
	}
	ts, err := strconv.ParseInt(tsTesto, 10, 64)
	if err != nil {
		return false, 0, nil
	}
	firma, err := base64.StdEncoding.DecodeString(firmaB64)
	if err != nil || len(firma) != ed25519.SignatureSize {
		return false, 0, nil
	}
	chiave, err := base64.StdEncoding.DecodeString(pk)
	if err != nil || len(chiave) != ed25519.PublicKeySize {
		return false, 0, nil
	}
	if !ed25519.Verify(chiave, MessaggioFirma(r.Metodo, r.Percorso, tsTesto, r.Corpo), firma) {
		return false, 0, nil
	}
	return true, ts, firma
}

// verifica controlla la firma di r con la chiave pk, l'ora e le ripetizioni,
// e se passa la segna come vista; "" se va bene, altrimenti il motivo.
func (r RichiestaFirmata) verifica(pk string, ora time.Time) string {
	if r.Intestazione == "" {
		return FirmaAssente
	}
	ok, ts, firma := r.firmaValida(pk)
	if !ok {
		return FirmaNonValida
	}
	if scarto := ora.Unix() - ts; scarto > int64(FinestraFirma/time.Second) || -scarto > int64(FinestraFirma/time.Second) {
		return FirmaFuoriTempo
	}
	if !firmeViste.nuova(string(firma), ts, ora.Unix()) {
		return FirmaRipetuta
	}
	return ""
}

// memoriaFirme ricorda le firme accettate finche' il loro ts e' nella
// finestra: e' in processo, quindi dopo un riavvio riparte vuota.
type memoriaFirme struct {
	mu       sync.Mutex
	scadenza map[string]int64
	pulita   int64
}

var firmeViste = &memoriaFirme{scadenza: map[string]int64{}}

// nuova segna la firma con ts e dice se non era gia' stata accettata.
func (m *memoriaFirme) nuova(firma string, ts, ora int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ora-m.pulita >= 60 {
		for k, s := range m.scadenza {
			if s < ora {
				delete(m.scadenza, k)
			}
		}
		m.pulita = ora
	}
	if s, c := m.scadenza[firma]; c && s >= ora {
		return false
	}
	m.scadenza[firma] = ts + int64(FinestraFirma/time.Second)
	return true
}
