package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

// Tipi degli eventi delle connessioni (README, Integrazione).
const (
	EventoConnessioneAperta = "connessione.aperta"
	EventoConnessioneChiusa = "connessione.chiusa"
	EventoConnessioneNota   = "connessione.nota"
	EventoFileTrasferiti    = "file.trasferiti"
	EventoTerminaleChiuso   = "terminale.chiuso"
	EventoPcNuovo           = "pc.nuovo"
	EventoPcCopia           = "pc.possibile_copia" //nolint:misspell // italiano, nome deciso dal compito (ADR-0026)
)

// Direzioni di file.trasferiti: il type del registro dei file, 0 quando il
// PC controllato manda i file al tecnico, 1 quando li riceve.
const (
	DirezioneDalPc = "dal_pc"
	DirezioneAlPc  = "al_pc"
)

// schemaEventi e' la versione del corpo degli eventi.
const schemaEventi = 1

// PcEvento e' il PC di un evento: ID, hostname e alias, mai uuid ne' chiavi.
type PcEvento struct {
	Id    string `json:"id"`
	Nome  string `json:"nome"`
	Alias string `json:"alias"`
}

// SessioneEvento e' la connessione di un evento di sessione: Tipo e' il
// type del registro, Tecnico il from_name; Fine e DurataS sono null finche'
// la connessione e' aperta. Mai IP, uuid, guid o session_id.
type SessioneEvento struct {
	ConnId  int64   `json:"conn_id"`
	Tipo    int     `json:"tipo"`
	Tecnico string  `json:"tecnico"`
	Inizio  string  `json:"inizio"`
	Fine    *string `json:"fine"`
	DurataS *int64  `json:"durata_s"`
	Nota    string  `json:"nota"`
}

// FileEvento e' un trasferimento di file: direzione e numero, mai nomi ne'
// percorsi, che restano nel pannello.
type FileEvento struct {
	Direzione string `json:"direzione"`
	Numero    int    `json:"numero"`
}

// VerificaEvento e' l'esito della verifica della catena di una
// trascrizione, come nel pannello: integra o no, e l'hash dell'ultimo
// blocco. Mai blocchi ne' dati.
type VerificaEvento struct {
	Integra bool   `json:"integra"`
	Hash    string `json:"hash"`
}

// CopiaEvento e' il numero di cambi di indirizzo di un PC forse copiato;
// gli indirizzi, che sono IP, non escono.
type CopiaEvento struct {
	Cambi int `json:"cambi"`
}

// corpoEvento e' il corpo comune degli eventi; i campi dopo Pc ci sono solo
// nei tipi che li usano.
type corpoEvento struct {
	Schema   int             `json:"schema"`
	Id       string          `json:"id"`
	Tipo     string          `json:"tipo"`
	Ora      string          `json:"ora"`
	Pc       PcEvento        `json:"pc"`
	Sessione *SessioneEvento `json:"sessione,omitempty"`
	File     *FileEvento     `json:"file,omitempty"`
	Verifica *VerificaEvento `json:"verifica,omitempty"`
	Copia    *CopiaEvento    `json:"copia,omitempty"`
}

type EventiService struct {
}

// AccodaSessione accoda l'evento tipo della connessione conn, col PC letto
// dal database; con l'integrazione spenta non fa niente.
func (es *EventiService) AccodaSessione(tipo string, conn *model.AuditConn) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	return accodaDelPc(conn.PeerId, corpoEvento{Tipo: tipo, Sessione: sessioneEvento(conn)})
}

// AccodaFile accoda file.trasferiti per il trasferimento f. Il registro
// conosce solo i type 0 (dal PC) e 1 (al PC): un altro type da' un warn e
// nessun evento.
func (es *EventiService) AccodaFile(f *model.AuditFile) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	var dir string
	switch f.Type {
	case 0:
		dir = DirezioneDalPc
	case 1:
		dir = DirezioneAlPc
	default:
		Logger.Warnf("file.trasferiti non accodato: type %d sconosciuto dal PC %s", f.Type, f.PeerId)
		return nil
	}
	return accodaDelPc(f.PeerId, corpoEvento{Tipo: EventoFileTrasferiti, File: &FileEvento{Direzione: dir, Numero: f.Num}})
}

// AccodaTerminale accoda terminale.chiuso per la sessione terminale conn,
// con l'esito v della verifica della sua trascrizione.
func (es *EventiService) AccodaTerminale(conn *model.AuditConn, v *model.AuditTerminalVerifica) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	return accodaDelPc(conn.PeerId, corpoEvento{Tipo: EventoTerminaleChiuso, Sessione: sessioneEvento(conn),
		Verifica: &VerificaEvento{Integra: v.Integra, Hash: v.Hash}})
}

// AccodaPcNuovo accoda pc.nuovo per il PC p, appena creato.
func (es *EventiService) AccodaPcNuovo(p *model.Peer) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	return accoda(corpoEvento{Tipo: EventoPcNuovo, Pc: PcEvento{Id: p.Id, Nome: p.Hostname, Alias: p.Alias}}, time.Now())
}

// AccodaCopia accoda pc.possibile_copia per il PC id con cambi cambi di
// indirizzo.
func (es *EventiService) AccodaCopia(id string, cambi int) error {
	if !Config.Integrazione.Attiva() {
		return nil
	}
	return accodaDelPc(id, corpoEvento{Tipo: EventoPcCopia, Copia: &CopiaEvento{Cambi: cambi}})
}

// accodaDelPc accoda c col PC id letto dal database.
func accodaDelPc(id string, c corpoEvento) error {
	pc, err := pcEvento(id)
	if err != nil {
		return err
	}
	c.Pc = pc
	return accoda(c, time.Now())
}

// pcEvento legge ID, hostname e alias del PC id; un PC che non c'e' ha solo
// l'ID.
func pcEvento(id string) (PcEvento, error) {
	pc := PcEvento{Id: id}
	p, err := AllService.PeerService.FindById(id)
	switch {
	case errors.Is(err, ErrNotFound):
		return pc, nil
	case err != nil:
		return pc, fmt.Errorf("PC dell'evento: %w", err)
	}
	pc.Nome, pc.Alias = p.Hostname, p.Alias
	return pc, nil
}

// sessioneEvento riduce la connessione conn ai campi che escono.
func sessioneEvento(conn *model.AuditConn) *SessioneEvento {
	inizio := time.Time(conn.CreatedAt)
	s := &SessioneEvento{ConnId: conn.ConnId, Tipo: conn.Type, Tecnico: conn.FromName,
		Inizio: ora(inizio), Nota: conn.Note}
	if conn.CloseTime > 0 {
		fine := time.Unix(conn.CloseTime, 0)
		f, d := ora(fine), max(0, conn.CloseTime-inizio.Unix())
		s.Fine, s.DurataS = &f, &d
	}
	return s
}

// ora scrive t in RFC 3339, UTC, al secondo.
func ora(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// accoda salva in coda l'evento c, creato all'ora adesso, con schema, id
// casuale e ora.
func accoda(c corpoEvento, adesso time.Time) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("id dell'evento: %w", err)
	}
	c.Schema, c.Id, c.Ora = schemaEventi, id.String(), ora(adesso)
	corpo, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("corpo dell'evento %s: %w", c.Tipo, err)
	}
	e := &model.Evento{EventoId: c.Id, Tipo: c.Tipo, Corpo: string(corpo),
		Creato: adesso.Unix(), Prossimo: adesso.Unix()}
	if err := DB.Create(e).Error; err != nil {
		return fmt.Errorf("evento %s in coda: %w", c.Tipo, err)
	}
	return nil
}

// Firma restituisce l'header X-Remotek-Evento-Firma del corpo, mandato al
// secondo UNIX t: "t=<t>,v1=<hex>", con v1 l'HMAC-SHA256 col segreto di
// "<t>." seguito dal corpo esatto.
func Firma(segreto string, t int64, corpo []byte) string {
	ts := strconv.FormatInt(t, 10)
	m := hmac.New(sha256.New, []byte(segreto))
	m.Write([]byte(ts + "."))
	m.Write(corpo)
	return "t=" + ts + ",v1=" + hex.EncodeToString(m.Sum(nil))
}

// SessioneDi restituisce i campi della connessione conn che escono verso
// l'integrazione, gli stessi degli eventi di sessione.
func SessioneDi(conn *model.AuditConn) *SessioneEvento { return sessioneEvento(conn) }
