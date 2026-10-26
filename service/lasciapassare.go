package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// DurataLasciapassare e' quanto vale un lasciapassare per aprire la
// sessione (ADR-0028).
const DurataLasciapassare = 60 * time.Second

// PrefissoLasciapassare precede il payload nel messaggio firmato.
const PrefissoLasciapassare = "remotek-lasciapassare-v1." //nolint:gosec // G101: separatore di dominio della firma, pubblico (README)

// Modi del lasciapassare: l'agente chiede i permessi di amministratore
// (ADR-0028) o no; il tecnico entrato con OIDC apre la sua sessione
// (ADR-0032). Il PC del client -5 accetta solo amministratore e ignora gli
// altri.
const (
	ModoAmministratore = "amministratore"
	ModoUtente         = "utente"
	ModoTecnico        = "tecnico"
)

// Capacita' del lasciapassare dell'agente (ADR-0033): cosa apre la
// sessione. Il terminale e' il lasciapassare v 1 di sempre; le altre danno
// il v 2, che il PC del client -5 non accetta.
const (
	CapacitaTerminale = "terminale"
	CapacitaExec      = "exec"
	CapacitaFile      = "file"
	CapacitaTunnel    = "tunnel"
	CapacitaSchermo   = "schermo"
)

// DurataTunnel e' la durata massima del tunnel, in secondi, quando
// l'agente non la chiede (ADR-0033).
const DurataTunnel = 3600

// Errori del rilascio: il testo e' l'ID del messaggio che ErrorErr trova
// nella catena.
var (
	ErrLasciapassareSpento = errors.New("PassOff")
	ErrNonAgente           = errors.New("PassNotAgent")
	ErrTecnicoNonAttivo    = errors.New("PassOwnerDisabled")
	ErrPcSconosciuto       = errors.New("PassUnknownPc")
	ErrPcFuoriRubrica      = errors.New("PassPcNotInAddressBook")
	ErrSenzaOidc           = errors.New("PassNeedsOidc")
	ErrAgenteSuTecnici     = errors.New("PassAgentOwnRoute")
)

// Errori del collegamento di un lasciapassare a una connessione: vanno solo
// nel log, mai al PC.
var (
	ErrNonceSconosciuto = errors.New("nonce di nessun lasciapassare")
	ErrNonceAltroPc     = errors.New("lasciapassare di un altro PC")
	ErrNonceUsato       = errors.New("lasciapassare gia' usato")
	ErrConnSenzaId      = errors.New("connessione senza conn_id")
)

// PayloadLasciapassare e' cio' che il lasciapassare dice al PC; i campi
// vanno nel JSON in quest'ordine, che fa parte del formato. Capacita, Porte
// e Durata ci sono solo col v 2 (Porte e' [] e Durata 0 tranne che per il
// tunnel): il v 1 e' quello di prima, byte per byte.
type PayloadLasciapassare struct {
	V        int    `json:"v"`
	Pc       string `json:"pc"`
	Agente   string `json:"agente"`
	Tecnico  string `json:"tecnico"`
	Modo     string `json:"modo"`
	Capacita string `json:"capacita"`
	Porte    []int  `json:"porte"`
	Durata   int    `json:"durata"`
	Emesso   int64  `json:"emesso"`
	Scade    int64  `json:"scade"`
	Nonce    string `json:"nonce"`
}

// MarshalJSON scrive il payload v 1 senza capacita, porte e durata, gli
// altri con porte sempre una lista.
func (p PayloadLasciapassare) MarshalJSON() ([]byte, error) {
	if p.V == 1 {
		return json.Marshal(struct {
			V       int    `json:"v"`
			Pc      string `json:"pc"`
			Agente  string `json:"agente"`
			Tecnico string `json:"tecnico"`
			Modo    string `json:"modo"`
			Emesso  int64  `json:"emesso"`
			Scade   int64  `json:"scade"`
			Nonce   string `json:"nonce"`
		}{p.V, p.Pc, p.Agente, p.Tecnico, p.Modo, p.Emesso, p.Scade, p.Nonce})
	}
	type v2 PayloadLasciapassare
	q := v2(p)
	if q.Porte == nil {
		q.Porte = []int{}
	}
	return json.Marshal(q)
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
// pc nel modo modo (ModoAmministratore o ModoUtente), con la capacita'
// capacita e, per il tunnel, le porte porte e la durata massima durata (in
// secondi), e lo registra, senza firma.
// Restituisce il lasciapassare e il tecnico dell'agente. Rifiuta con
// ErrLasciapassareSpento senza chiave, con ErrNonAgente se agente non e' un
// agente AI attivo, con ErrTecnicoNonAttivo se il suo tecnico non c'e' o e'
// disattivato, con ErrPcSconosciuto se pc non e' in peers, con
// ErrPcFuoriRubrica (avvolto con PC, tecnico e agente, che il warn della
// risposta riporta) se pc non e' nella rubrica del tecnico: un agente
// lavora solo sui PC su cui lavora il suo tecnico.
func (ls *LasciapassareService) Rilascia(agente *model.User, pc, modo, capacita string, porte []int, durata int, adesso time.Time) (string, *model.User, error) {
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
	if err := pcNoto(pc); err != nil {
		return "", nil, err
	}
	switch ok, err := AllService.AddressBookService.NellaRubrica(tecnico, pc); {
	case err != nil:
		return "", nil, fmt.Errorf("rubrica del tecnico %d: %w", tecnico.Id, err)
	case !ok:
		return "", nil, fmt.Errorf("lasciapassare rifiutato: il PC %s non e' nella rubrica del tecnico %s (agente %s): %w",
			pc, tecnico.Username, agente.Username, ErrPcFuoriRubrica)
	}
	lp, err := firmaERegistra(chiave, pc, agente, tecnico, modo, capacita, porte, durata, adesso)
	return lp, tecnico, err
}

// RilasciaTecnico firma, all'ora adesso, il lasciapassare del tecnico u per
// il PC pc, nel modo ModoTecnico, e lo registra con AgenteId 0 (ADR-0032).
// Un tecnico entrato con OIDC (daOidc) lo riceve per qualunque PC in peers,
// senza la rubrica; col login con password solo un amministratore in
// app.pwd-login-eccezioni (l'admin di emergenza), ed emergenza e' vero.
// Rifiuta con ErrLasciapassareSpento senza chiave, con ErrAgenteSuTecnici
// un agente AI (che ha la sua rotta), con ErrSenzaOidc gli altri entrati con
// la password, con ErrPcSconosciuto se pc non e' in peers.
func (ls *LasciapassareService) RilasciaTecnico(u *model.User, daOidc bool, pc string, adesso time.Time) (lp string, emergenza bool, err error) {
	chiave := Config.Agente.Chiave()
	switch {
	case chiave == nil:
		return "", false, ErrLasciapassareSpento
	case u.AgenteDi != 0:
		return "", false, ErrAgenteSuTecnici
	case u.Status != model.COMMON_STATUS_ENABLE:
		return "", false, ErrSenzaOidc
	case !daOidc:
		if !AllService.UserService.IsAdmin(u) || !Config.App.EccezionePwdLogin(u.Username) {
			return "", false, ErrSenzaOidc
		}
		emergenza = true
	}
	if err := pcNoto(pc); err != nil {
		return "", false, err
	}
	lp, err = firmaERegistra(chiave, pc, nil, u, ModoTecnico, CapacitaTerminale, nil, 0, adesso)
	return lp, emergenza, err
}

// pcNoto restituisce ErrPcSconosciuto se pc non e' in peers.
func pcNoto(pc string) error {
	if _, err := AllService.PeerService.FindById(pc); errors.Is(err, ErrNotFound) {
		return ErrPcSconosciuto
	} else if err != nil {
		return fmt.Errorf("PC del lasciapassare: %w", err)
	}
	return nil
}

// firmaERegistra firma il lasciapassare per pc, dell'agente (nil per un
// tecnico) e del tecnico nel modo modo, con la capacita' capacita, le
// porte porte e la durata durata (v 1 col terminale, v 2 con le altre), e
// ne registra il nonce con capacita', porte e durata.
func firmaERegistra(chiave ed25519.PrivateKey, pc string, agente, tecnico *model.User, modo, capacita string, porte []int, durata int, adesso time.Time) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("nonce del lasciapassare: %w", err)
	}
	p := PayloadLasciapassare{V: 1, Pc: pc, Tecnico: tecnico.Username, Modo: modo,
		Emesso: adesso.Unix(), Scade: adesso.Add(DurataLasciapassare).Unix(), Nonce: base64.RawURLEncoding.EncodeToString(b)}
	r := &model.Lasciapassare{Nonce: p.Nonce, Pc: pc, TecnicoId: tecnico.Id, Modo: modo, Capacita: capacita,
		Emesso: p.Emesso, Scade: p.Scade, Creato: adesso.Unix()}
	if capacita != CapacitaTerminale {
		p.V, p.Capacita, p.Porte, p.Durata, r.Durata = 2, capacita, porte, durata, durata
		testo := make([]string, len(porte))
		for i, n := range porte {
			testo[i] = strconv.Itoa(n)
		}
		r.Porte = strings.Join(testo, ",")
	}
	if agente != nil {
		p.Agente, r.AgenteId = agente.Username, agente.Id
	}
	lp, err := FirmaLasciapassare(chiave, p)
	if err != nil {
		return "", err
	}
	if err := DB.Create(r).Error; err != nil {
		return "", fmt.Errorf("registro del lasciapassare: %w", err)
	}
	return lp, nil
}

// Lega collega alla connessione conn il lasciapassare col nonce nonce, se e'
// stato rilasciato per il PC di conn e non ha ancora aperto una sessione:
// segna sul lasciapassare la riga di conn (AuditConnId) e il suo conn_id, e
// su conn, anche nel database, l'agente (vuoto per il lasciapassare di un
// tecnico) e il tecnico verificati. I permessi di amministratore no: li dice
// il PC (campo amministratore di /api/audit/conn). Altrimenti non cambia
// niente e restituisce ErrNonceSconosciuto, ErrNonceAltroPc, ErrNonceUsato
// o ErrConnSenzaId.
//
// Scade non si guarda, ed e' voluto: il lasciapassare lo verifica il PC
// quando apre la sessione (60 secondi), ma il PC puo' mandare il nonce
// all'audit dopo il clic del cliente, anche minuti dopo. Il legame vale una
// volta sola, ed e' quello che conta.
func (ls *LasciapassareService) Lega(nonce string, conn *model.AuditConn) error {
	var agente, tecnico model.User
	err := DB.Transaction(func(tx *gorm.DB) error {
		lp := &model.Lasciapassare{}
		if err := tx.Where("nonce = ?", nonce).First(lp).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNonceSconosciuto
		} else if err != nil {
			return fmt.Errorf("lasciapassare: %w", err)
		}
		switch {
		case lp.Pc != conn.PeerId:
			return ErrNonceAltroPc
		case lp.AuditConnId != 0:
			return ErrNonceUsato
		case conn.ConnId == 0:
			return ErrConnSenzaId
		}
		r := tx.Model(lp).Where("audit_conn_id = 0").
			Updates(map[string]any{"audit_conn_id": conn.Id, "conn_id": conn.ConnId})
		if r.Error != nil {
			return fmt.Errorf("legame del lasciapassare: %w", r.Error)
		}
		if r.RowsAffected == 0 {
			return ErrNonceUsato
		}
		if lp.AgenteId != 0 { // 0: lasciapassare di un tecnico, senza agente
			if err := tx.First(&agente, lp.AgenteId).Error; err != nil {
				return fmt.Errorf("agente %d del lasciapassare: %w", lp.AgenteId, err)
			}
		}
		if err := tx.First(&tecnico, lp.TecnicoId).Error; err != nil {
			return fmt.Errorf("tecnico %d del lasciapassare: %w", lp.TecnicoId, err)
		}
		v := map[string]any{"agente_lasciapassare": agente.Username, "tecnico_lasciapassare": tecnico.Username}
		if err := tx.Model(&model.AuditConn{}).Where("id = ?", conn.Id).Updates(v).Error; err != nil {
			return fmt.Errorf("agente del lasciapassare sulla connessione: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	conn.AgenteLasciapassare, conn.TecnicoLasciapassare = agente.Username, tecnico.Username
	return nil
}
