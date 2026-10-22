package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// DurataLasciapassare e' quanto vale un lasciapassare per aprire la
// sessione (ADR-0028).
const DurataLasciapassare = 60 * time.Second

// PrefissoLasciapassare precede il payload nel messaggio firmato.
const PrefissoLasciapassare = "remotek-lasciapassare-v1." //nolint:gosec // G101: separatore di dominio della firma, pubblico (README)

// ModoAmministratore e' il modo dei lasciapassare: l'agente chiede i
// permessi di amministratore.
const ModoAmministratore = "amministratore"

// Errori del rilascio: il testo e' l'ID del messaggio che ErrorErr trova
// nella catena.
var (
	ErrLasciapassareSpento = errors.New("PassOff")
	ErrNonAgente           = errors.New("PassNotAgent")
	ErrTecnicoNonAttivo    = errors.New("PassOwnerDisabled")
	ErrPcSconosciuto       = errors.New("PassUnknownPc")
)

// PayloadLasciapassare e' cio' che il lasciapassare dice al PC; i campi
// vanno nel JSON in quest'ordine, che fa parte del formato.
type PayloadLasciapassare struct {
	V       int    `json:"v"`
	Pc      string `json:"pc"`
	Agente  string `json:"agente"`
	Tecnico string `json:"tecnico"`
	Modo    string `json:"modo"`
	Emesso  int64  `json:"emesso"`
	Scade   int64  `json:"scade"`
	Nonce   string `json:"nonce"`
}

// FirmaLasciapassare restituisce il lasciapassare di p:
// base64url(payload) + "." + base64url(firma), senza padding, con payload
// il JSON compatto di p e firma Ed25519(chiave, PrefissoLasciapassare +
// payload).
func FirmaLasciapassare(chiave ed25519.PrivateKey, p PayloadLasciapassare) (string, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("payload del lasciapassare: %w", err)
	}
	firma := ed25519.Sign(chiave, append([]byte(PrefissoLasciapassare), payload...))
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(firma), nil
}

type LasciapassareService struct {
}

// Rilascia firma, all'ora adesso, il lasciapassare dell'agente per il PC
// pc e lo registra, senza firma. Restituisce il lasciapassare e il tecnico
// dell'agente. Rifiuta con ErrLasciapassareSpento senza chiave, con
// ErrNonAgente se agente non e' un agente AI attivo, con
// ErrTecnicoNonAttivo se il suo tecnico non c'e' o e' disattivato, con
// ErrPcSconosciuto se pc non e' in peers.
func (ls *LasciapassareService) Rilascia(agente *model.User, pc string, adesso time.Time) (string, *model.User, error) {
	chiave := Config.Agente.Chiave()
	if chiave == nil {
		return "", nil, ErrLasciapassareSpento
	}
	if agente.AgenteDi == 0 || agente.Status != model.COMMON_STATUS_ENABLE {
		return "", nil, ErrNonAgente
	}
	tecnico, err := AllService.UserService.InfoById(agente.AgenteDi)
	switch {
	case errors.Is(err, ErrNotFound):
		return "", nil, ErrTecnicoNonAttivo
	case err != nil:
		return "", nil, fmt.Errorf("tecnico dell'agente %d: %w", agente.Id, err)
	case tecnico.Status != model.COMMON_STATUS_ENABLE:
		return "", nil, ErrTecnicoNonAttivo
	}
	if _, err := AllService.PeerService.FindById(pc); errors.Is(err, ErrNotFound) {
		return "", nil, ErrPcSconosciuto
	} else if err != nil {
		return "", nil, fmt.Errorf("PC del lasciapassare: %w", err)
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("nonce del lasciapassare: %w", err)
	}
	p := PayloadLasciapassare{V: 1, Pc: pc, Agente: agente.Username, Tecnico: tecnico.Username, Modo: ModoAmministratore,
		Emesso: adesso.Unix(), Scade: adesso.Add(DurataLasciapassare).Unix(), Nonce: base64.RawURLEncoding.EncodeToString(b)}
	lp, err := FirmaLasciapassare(chiave, p)
	if err != nil {
		return "", nil, err
	}
	r := &model.Lasciapassare{Nonce: p.Nonce, Pc: pc, AgenteId: agente.Id, TecnicoId: tecnico.Id,
		Emesso: p.Emesso, Scade: p.Scade, Creato: adesso.Unix()}
	if err := DB.Create(r).Error; err != nil {
		return "", nil, fmt.Errorf("registro del lasciapassare: %w", err)
	}
	return lp, tecnico, nil
}
